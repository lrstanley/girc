package girc

import (
	"math/rand"
	"time"
)

func randSleep() {
	rand.Seed(time.Now().UnixNano())
	time.Sleep(time.Duration(rand.Intn(25)) * time.Millisecond)
}
