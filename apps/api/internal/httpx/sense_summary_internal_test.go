package httpx

import (
	"strings"
	"testing"
)

func TestParseSenseSummary(t *testing.T) {
	got, err := parseSenseSummary("```json\n{\"overview\":\" 流れ \",\"good\":[\"a\",\"\",\"b\",\"c\",\"d\"],\"next\":[\"" + strings.Repeat("あ", 120) + "\"]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got.Overview != "流れ" || len(got.Good) != 3 || got.Good[1] != "b" || got.Concerns == nil || len([]rune(got.Next[0])) != maxSenseSummaryItem {
		t.Fatalf("got = %+v", got)
	}
	if _, err := parseSenseSummary(`{"overview":"  ","good":["  "],"concerns":[],"next":[]}`); err == nil {
		t.Fatal("empty overview accepted")
	}
	// 配列で包んで返ってきても、1つ目をまとめとして使う。
	got, err = parseSenseSummary(`[{"overview":"包まれた","good":["a"],"concerns":[],"next":[]}]`)
	if err != nil || got.Overview != "包まれた" || len(got.Good) != 1 {
		t.Fatalf("wrapped = %+v %v", got, err)
	}
	if _, err := parseSenseSummary(`[]`); err == nil {
		t.Fatal("empty array accepted")
	}
	if _, err := parseSenseSummary("まとめです"); err == nil {
		t.Fatal("text accepted")
	}
}

// 読めなかった返事の中身(メモをもとに書かれる)は、エラーに入れない。ログに出るため。
func TestParseSenseSummaryErrorHidesReply(t *testing.T) {
	for _, raw := range []string{"左ひざが痛くて走れなかった", `[{"overview":"左ひざが痛い"`, `{"overview":"左ひざが痛い"`} {
		_, err := parseSenseSummary(raw)
		if err == nil {
			t.Fatalf("%q: エラーになるはず", raw)
		}
		if strings.Contains(err.Error(), "左ひざ") {
			t.Fatalf("返事の中身がエラーに入っている: %v", err)
		}
	}
}

func TestParseSenseSummaryUsesReturnedItemsForEmptyOverview(t *testing.T) {
	for _, field := range []string{"good", "concerns", "next"} {
		got, err := parseSenseSummary(`{"overview":" ","` + field + `":["  AIが返した文章  "]}`)
		if err != nil || got.Overview != "AIが返した文章" {
			t.Fatalf("%s: %#v, %v", field, got, err)
		}
	}
	got, err := parseSenseSummary(`{"overview":"","good":["安定しました","別の感覚"],"concerns":["力みました"],"next":["リズムを意識しましょう"]}`)
	if err != nil || got.Overview != "安定しました\n力みました\nリズムを意識しましょう" {
		t.Fatalf("%#v, %v", got, err)
	}
}
