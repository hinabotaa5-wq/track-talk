package httpx

// SetReviewPaymentMethodsForTest は、動画添削とその追加相談で受け付ける支払い方法をテストの間だけ差し替え、
// 元に戻す関数を返す（httpx_test から使う）。
func SetReviewPaymentMethodsForTest(methods []string) (restore func()) {
	old := reviewPaymentMethods
	reviewPaymentMethods = methods
	return func() { reviewPaymentMethods = old }
}
