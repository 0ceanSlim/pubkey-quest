package savecodec

// migrations[v] turns a v save into a v+1 save, working on the raw compact
// JSON so each step only needs to know its own two versions. Every step must
// be a pure function with a round-trip test (codec_test.go) — saves on relays
// can be dug up years later and must still load.
//
// To change the format: bump Version, add migrations[Version-1], and leave
// every older entry exactly as it is.
var migrations = map[int]func([]byte) ([]byte, error){
	// v1 is the first relay format: nothing to migrate from yet.
}
