package u

import (
	"log"

	"git.oxl.at/micro-queue/internal/config"
)

func LogDebug(msg string) {
	if config.MODE_DEV || config.MODE_DEBUG {
		log.Println(msg)
	}
}
