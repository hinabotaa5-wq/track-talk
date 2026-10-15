// budgetstop は Google Cloud の予算通知（Pub/Sub の push）を受け取り、
// 使った金額が予算に達していたら API の Cloud Run サービスを外から呼べなくする。
// 再開は手作業で `gcloud run services update <サービス> --ingress all` を実行する。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const internalOnly = "INGRESS_TRAFFIC_INTERNAL_ONLY"

type stopper struct {
	// projects/<PROJECT>/locations/<REGION>/services/<SERVICE>
	service string
	runAPI  string
	token   func(context.Context) (string, error)
	client  *http.Client
}

type pushRequest struct {
	Message struct {
		Data []byte `json:"data"`
	} `json:"message"`
}

type budgetNotice struct {
	CostAmount   float64 `json:"costAmount"`
	BudgetAmount float64 `json:"budgetAmount"`
}

func main() {
	service := os.Getenv("TARGET_SERVICE")
	if service == "" {
		slog.Error("config", "err", "TARGET_SERVICE is required")
		os.Exit(1)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	s := stopper{
		service: service,
		runAPI:  "https://run.googleapis.com/v2/",
		token:   metadataToken(client),
		client:  client,
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: s, ReadHeaderTimeout: 5 * time.Second}
	slog.Info("listen", "addr", srv.Addr, "target", service)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}

func (s stopper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var push pushRequest
	if err := json.NewDecoder(r.Body).Decode(&push); err != nil {
		// 形式が違う通知は再送しても直らないので 2xx で受け取って捨てる。
		slog.Error("decode push", "err", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var notice budgetNotice
	if err := json.Unmarshal(push.Message.Data, &notice); err != nil {
		slog.Error("decode budget notice", "err", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if notice.BudgetAmount <= 0 || notice.CostAmount < notice.BudgetAmount {
		slog.Info("within budget", "cost", notice.CostAmount, "budget", notice.BudgetAmount)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.stop(r.Context()); err != nil {
		// 失敗時は 5xx を返し、Pub/Sub に再送させる。
		slog.Error("stop service", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	slog.Warn("budget exceeded, service closed to the internet", "cost", notice.CostAmount, "budget", notice.BudgetAmount, "service", s.service)
	w.WriteHeader(http.StatusOK)
}

func (s stopper) stop(ctx context.Context) error {
	token, err := s.token(ctx)
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}
	body, _ := json.Marshal(map[string]string{"ingress": internalOnly})
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, s.runAPI+s.service+"?updateMask=ingress", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("cloud run api: %s: %s", res.Status, msg)
	}
	return nil
}

// metadataToken は Cloud Run 上で、サービスに付けたサービスアカウントのアクセストークンを取る。
func metadataToken(client *http.Client) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Metadata-Flavor", "Google")
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("metadata server: %s", res.Status)
		}
		var out struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return "", err
		}
		if out.AccessToken == "" {
			return "", errors.New("metadata server: empty token")
		}
		return out.AccessToken, nil
	}
}
