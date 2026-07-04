package auth

// SetFastHashParams drops the argon2id cost parameters to the cheapest
// sensible values for the duration of a test and restores them via
// tb.Cleanup. Hashing at production cost is tens of milliseconds plus a
// 64 MiB allocation per call, which multiplies across every seeded user
// and password round-trip in the api test suites. Verification is
// unaffected: VerifyPassword reads its parameters from the PHC hash
// string, so hashes made cheap verify cheap and production hashes still
// verify at full cost.
//
// The parameter is the subset of testing.TB this needs — importing
// package testing from non-test code is wrong, and a caller outside a
// test has no business calling this.
//
// Not safe to call from parallel tests: the parameters are package
// globals. Keep at least one round-trip test on the production values so
// the real cost path stays exercised.
func SetFastHashParams(tb interface{ Cleanup(func()) }) {
	prevTime, prevMemory := argonTime, argonMemory
	argonTime, argonMemory = 1, 8*1024
	tb.Cleanup(func() {
		argonTime, argonMemory = prevTime, prevMemory
	})
}
