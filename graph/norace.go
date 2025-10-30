//go:build !race

package graph

// raceEnabled is false when tests are run without the race detector.
const raceEnabled = false
