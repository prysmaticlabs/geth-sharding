// Package logutil creates a Multi writer instance that
// write all logs that are written to stdout.
package logutil

import (
	"fmt"
	"io"
	"os"
	"time"
	"github.com/sirupsen/logrus"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
)

// ConfigurePersistentLogging adds a log-to-file writer. File content is identical to stdout.
func ConfigurePersistentLogging(logFileName string) error {
	logrus.WithField("logFileName", logFileName).Info("Logs will be made persistent")
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	mw := io.MultiWriter(os.Stdout, f)
	logrus.SetOutput(mw)

	logrus.Info("File logging initialized")
	return nil
}

func CountdownToGenesis(genesisTime time.Time, secondsCount int) {
	ticker := time.NewTicker(time.Duration(secondsCount) * time.Second)

	for {
		select {
		case <-time.NewTimer(genesisTime.Sub(roughtime.Now()) + 1).C:
			fmt.Printf("genesis time\n")
			return

		case <-ticker.C:
			fmt.Printf("%02d minutes to genesis!\n", genesisTime.Sub(roughtime.Now()).Round(time.Minute)/time.Minute+1)
		}
	}
}
