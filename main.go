package main

import (
	"log"
	"ncryptagent/keyman"
	"ncryptagent/ui"
	"os"
)

func main() {
	if keyman.NCryptSignerWorkerRequested(os.Args) {
		if err := keyman.RunNCryptSignerWorker(os.Stdin, os.Stdout); err != nil {
			log.Printf("NCrypt signer worker failed: %v", err)
			os.Exit(1)
		}
		return
	}

	ui.RunUI()
}
