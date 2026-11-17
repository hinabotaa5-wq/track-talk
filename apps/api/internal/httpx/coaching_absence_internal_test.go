package httpx

import (
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

func dateUTC(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// 30日間の1ヶ月プラン（10,000円）。開始 11/01、最終日 11/30（両端を含む）。
func absenceSupportContract() db.CoachingContract {
	zero := int16(0)
	service := "competition_support"
	start := time.Date(2026, 11, 1, 9, 0, 0, 0, jst.Zone)
	comp := dateUTC(2026, 11, 30)
	ends := time.Date(2026, 12, 1, 0, 0, 0, 0, jst.Zone)
	return db.CoachingContract{
		PriceYen: 10000, VideoReviews: &zero, StartsAt: &start, CompetitionDate: &comp, EndsAt: &ends,
		FulfillmentVersion: 1, FulfillmentState: "open", Status: "active", ServiceType: &service,
	}
}

func withAgreedAbsence(ct db.CoachingContract, from, ret time.Time) db.CoachingContract {
	days := int32(ret.Sub(from).Hours() / 24)
	ct.AbsenceFrom, ct.AbsenceReturnOn, ct.AbsenceDays = &from, &ret, days
	e := ct.EndsAt.Add(time.Duration(days) * 24 * time.Hour)
	ct.EndsAt = &e
	return ct
}

// 復帰日は不在日に含めない（11/20〜11/22不在・11/23復帰なら3日）。選手がその日に何も送らなかったことは数え方に関係しない。
func TestAbsenceReturnDayIsNotAbsent(t *testing.T) {
	ct := absenceSupportContract()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, jst.Zone)
	newEnds, rule := absenceOffer(ct, dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), now, nil, true)
	if rule != nil {
		t.Fatal(rule)
	}
	if want := ct.EndsAt.Add(3 * 24 * time.Hour); !newEnds.Equal(want) {
		t.Fatalf("new end %v want %v", newEnds, want)
	}
	agreed := withAgreedAbsence(ct, dateUTC(2026, 11, 20), dateUTC(2026, 11, 23))
	for day, want := range map[int]int64{19: 0, 20: 1, 21: 2, 22: 3, 23: 3, 30: 3} {
		if got := absentDaysThrough(agreed, dateUTC(2026, 11, day)); got != want {
			t.Fatalf("11/%d absent %d want %d", day, got, want)
		}
	}
}

// 30日間・不在3日を合意・開始から暦日で25日目（不在3日を含む）に終了 → 経過22日・未経過8日・返金は価格÷30×8（円未満切り捨て）。
func TestAbsenceSupportSettlementExcludesAbsentDays(t *testing.T) {
	ct := withAgreedAbsence(absenceSupportContract(), dateUTC(2026, 11, 11), dateUTC(2026, 11, 14))
	day25 := dateUTC(2026, 11, 25)
	elapsed, total := supportDays(ct, day25)
	if elapsed != 22 || total != 30 {
		t.Fatalf("days %d/%d", elapsed, total)
	}
	if got := supportUnprovidedAsOf(ct, day25); got != 10000*8/30 {
		t.Fatalf("refund %d", got)
	}
	// 延長しても分母は当初の30日のまま。終了日が後ろ倒しになった後の日付でも、経過は期間日数を超えない。
	if e, total := supportDays(ct, dateUTC(2026, 12, 3)); e != 30 || total != 30 {
		t.Fatalf("after extended end %d/%d", e, total)
	}
	// 延長を使わない契約の精算は、従来と1円も変わらない。
	plain := absenceSupportContract()
	if e, total := supportDays(plain, day25); e != 25 || total != 30 {
		t.Fatalf("plain days %d/%d", e, total)
	}
	if got := supportUnprovidedAsOf(plain, day25); got != 10000*5/30 {
		t.Fatalf("plain refund %d", got)
	}
	// 不在日がまだ来ていない日に終えるなら、不在日は控除しない。
	if e, _ := supportDays(ct, dateUTC(2026, 11, 8)); e != 8 {
		t.Fatalf("before absence %d", e)
	}
	// コーチ報酬は経過分（不在日を除く）の75%。
	if got := supportEarnedAsOf(ct, day25); got != supportCoachShare(10000-10000*8/30) {
		t.Fatalf("earned %d", got)
	}
}

// 期間プランの残り日数返金。30日間・不在3日を合意（終了日は33日後）・暦日25日目に終了 → 残り8日。
func TestAbsencePeriodRemainingRefund(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, jst.Zone)
	ends := start.Add(30 * 24 * time.Hour)
	ct := db.CoachingContract{StartsAt: &start, EndsAt: &ends, DurationDays: 30, Status: "active"}
	pay := db.CoachingPayment{AmountYen: 10000}
	at := start.Add(25 * 24 * time.Hour)
	if got := remainingRefund(ct, pay, at); got != 10000*5/30 {
		t.Fatalf("plain %d", got)
	}
	ext := withAgreedAbsence(ct, dateUTC(2026, 11, 11), dateUTC(2026, 11, 14))
	if got := remainingRefund(ext, pay, at); got != 10000*8/30 {
		t.Fatalf("extended %d", got)
	}
	// 不在日がまだ来ていないときは、控除せず、当初の終了日までの残りで数える（分母は30日のまま、上限は期間日数）。
	if got := remainingRefund(ext, pay, start.Add(5*24*time.Hour)); got != 10000*25/30 {
		t.Fatalf("before absence %d", got)
	}
	if got := remainingRefund(ext, pay, start); got != 10000 {
		t.Fatalf("at start %d", got)
	}
	// 延長後の終了日を過ぎたら0円。
	if got := remainingRefund(ext, pay, ext.EndsAt.Add(time.Hour)); got != 0 {
		t.Fatalf("after end %d", got)
	}
}

func TestAbsenceOfferLimits(t *testing.T) {
	ct := absenceSupportContract()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, jst.Zone)
	code := func(from, ret time.Time, c db.CoachingContract, goal *time.Time) string {
		_, rule := absenceOffer(c, from, ret, now, goal, true)
		if rule == nil {
			return ""
		}
		return rule.code
	}
	// 7日までは提示できる。8日目以降は成立しない（累計7日）。
	if got := code(dateUTC(2026, 11, 12), dateUTC(2026, 11, 19), ct, nil); got != "" {
		t.Fatalf("7 days %q", got)
	}
	if got := code(dateUTC(2026, 11, 12), dateUTC(2026, 11, 20), ct, nil); got != "absence_limit" {
		t.Fatalf("8 days %q", got)
	}
	// 2回目の不在（復帰遅延を含む）は提示しない。
	second := withAgreedAbsence(ct, dateUTC(2026, 11, 11), dateUTC(2026, 11, 14))
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 21), second, nil); got != "absence_second" {
		t.Fatalf("second %q", got)
	}
	// 延長後の終了日が目標の大会日を過ぎる場合は提示しない（最終日 11/30 に3日足すと 12/3）。
	goal := dateUTC(2026, 12, 2)
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), ct, &goal); got != "absence_past_goal" {
		t.Fatalf("goal %q", got)
	}
	okGoal := dateUTC(2026, 12, 3)
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), ct, &okGoal); got != "" {
		t.Fatalf("goal ok %q", got)
	}
	// 事前の申告だけ。過去の日、復帰日が開始日以前、契約の期間外は提示しない。
	for name, c := range map[string]string{
		"past":     code(dateUTC(2026, 11, 9), dateUTC(2026, 11, 12), ct, nil),
		"reversed": code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 20), ct, nil),
		"outside":  code(dateUTC(2026, 11, 29), dateUTC(2026, 12, 5), ct, nil),
	} {
		if c != "absence_period_invalid" && !(name == "outside" && c == "absence_limit") {
			t.Fatalf("%s %q", name, c)
		}
	}
	// 対象外: 1回プラン・終了済み・最初の動画待ちの契約。
	single := ct
	single.FulfillmentVersion, single.ServiceType, single.PlanKind = 0, nil, "single"
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), single, nil); got != "absence_not_eligible" {
		t.Fatalf("single %q", got)
	}
	ended := ct
	ended.Status = "ended"
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), ended, nil); got != "absence_not_eligible" {
		t.Fatalf("ended %q", got)
	}
	period := db.CoachingContract{Status: "active", FulfillmentVersion: 0, PlanKind: "period", FulfillmentState: "open", StartsAt: ct.StartsAt, EndsAt: ct.EndsAt, DurationDays: 30}
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), period, nil); got != "" {
		t.Fatalf("period %q", got)
	}
	waiting := period
	waiting.StartOnFirstVideo = true
	if got := code(dateUTC(2026, 11, 20), dateUTC(2026, 11, 23), waiting, nil); got != "absence_not_eligible" {
		t.Fatalf("waiting %q", got)
	}
}
