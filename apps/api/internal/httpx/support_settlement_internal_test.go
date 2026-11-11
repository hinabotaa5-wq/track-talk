package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 10日間・9,000円の契約で、3日目に終えると未経過7日分＝6,300円を返金し、コーチ報酬は経過3日分2,700円の75%＝2,025円。
func TestSupportSettlementByElapsedDays(t *testing.T) {
	zero := int16(0)
	start, comp := time.Date(2026, 12, 1, 9, 0, 0, 0, jst.Zone), time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	service := "competition_support"
	ct := db.CoachingContract{PriceYen: 9000, VideoReviews: &zero, StartsAt: &start, CompetitionDate: &comp, FulfillmentVersion: 1, ServiceType: &service}
	if !supportConsultContract(ct) {
		t.Fatal("consult contract not detected")
	}
	day3 := time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)
	if e, total := supportDays(ct, day3); e != 3 || total != 10 {
		t.Fatalf("days %d/%d", e, total)
	}
	if got := supportUnprovidedAsOf(ct, day3); got != 6300 {
		t.Fatalf("refund %d", got)
	}
	if got := supportEarnedAsOf(ct, day3); got != 2025 {
		t.Fatalf("earned %d", got)
	}
	// 選手の都合・やむを得ない事情は、終了の日までの経過日数で確定する（返金額によらない）。
	// 大会の中止延期は新規の終了理由としては受け付けないが、既存データの解釈は同じ日割りで残す。
	for _, cause := range []string{supportEndPlayer, supportEndUnavoidable, supportEndCancelled} {
		c := cause
		ct.SupportEndCause, ct.SupportEndOn = &c, &day3
		if got := supportCoachEarned(ct, 9000, 9000); got != 2025 {
			t.Fatalf("%s earned %d", cause, got)
		}
	}
	// 終了の日が記録されていれば、その日を基準にする（その日も経過扱い）。
	announced := time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)
	ct.SupportEndOn = &announced
	if got := supportUnprovidedAsOf(ct, supportAsOf(ct, time.Now())); got != 7200 {
		t.Fatalf("end-on refund %d", got)
	}
	// コーチ都合は0円。
	fault := supportEndCoachFault
	ct.SupportEndCause = &fault
	if got := supportCoachEarned(ct, 9000, 9000); got != 0 {
		t.Fatalf("fault earned %d", got)
	}
	// 終わり方の記録が無い（期間を終えた等）は、払われた額の75%。
	ct.SupportEndCause, ct.SupportEndOn = nil, nil
	if got := supportCoachEarned(ct, 9000, 0); got != 6750 {
		t.Fatalf("natural earned %d", got)
	}
	// 円未満は切り捨て。10,000円・3日間の1日目: 経過分3,334円（10,000−6,666）の75%＝2,500円。
	comp3 := time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)
	ct3 := db.CoachingContract{PriceYen: 10000, VideoReviews: &zero, StartsAt: &start, CompetitionDate: &comp3}
	if got := supportEarnedAsOf(ct3, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)); got != 2500 {
		t.Fatalf("floor %d", got)
	}
}

func TestSupportSettleAdvancesAndClawbackCap(t *testing.T) {
	for _, c := range []struct {
		name                           string
		earned, paid, unpaid, refunded int32
		final, clawback                int32
		drop                           bool
	}{
		{"確定額が仮払いより多ければ差額を記録", 7200, 2400, 1000, 0, 3800, 0, false},
		{"確定額と仮払いが同じなら0円", 2400, 2400, 0, 6600, 0, 0, false},
		{"送金前の仮払いは消して差し引く", 2500, 2000, 1000, 0, 500, 0, true},
		{"コーチ都合: 送金済みの仮払いを返還", 0, 2400, 800, 9000, 0, 2400, true},
		{"返還は返金額の75%が上限", 0, 5000, 0, 3000, 0, 2250, true},
	} {
		final, claw, drop := supportSettle(c.earned, c.paid, c.unpaid, c.refunded)
		if final != c.final || claw != c.clawback || drop != c.drop {
			t.Errorf("%s: final %d claw %d drop %v", c.name, final, claw, drop)
		}
	}
}

func TestSupportAdvanceThroughIsPreviousMonthEnd(t *testing.T) {
	through, end := supportAdvanceThrough(time.Date(2027, 1, 1, 0, 30, 0, 0, jst.Zone))
	if through.Format(time.DateOnly) != "2026-12-31" || !end.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, jst.Zone)) {
		t.Fatalf("through %s end %s", through, end)
	}
}
