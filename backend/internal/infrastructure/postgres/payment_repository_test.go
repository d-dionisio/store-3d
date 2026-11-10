package postgres

import "testing"

func TestPaymentStatusMapping(t *testing.T) {
	cases := []struct {
		provider, detail string
		refunded         int64
		payment, attempt string
	}{{"pending", "", 0, "pending", "pending"}, {"in_process", "", 0, "processing", "processing"}, {"approved", "accredited", 0, "paid", "paid"}, {"rejected", "cc_rejected", 0, "failed", "failed"}, {"cancelled", "expired", 0, "cancelled", "expired"}, {"refunded", "refunded", 100, "refunded", "paid"}, {"approved", "partially_refunded", 50, "partially_refunded", "paid"}}
	for _, c := range cases {
		if got := internalStatus(c.provider, c.refunded); got != c.payment {
			t.Errorf("payment %s: got %s want %s", c.provider, got, c.payment)
		}
		if got := attemptStatus(c.provider, c.detail, c.refunded); got != c.attempt {
			t.Errorf("attempt %s: got %s want %s", c.provider, got, c.attempt)
		}
	}
}
