// Copyright (c) 2022 Target Brands, Inc. All rights reserved.
//
// Use of this source code is governed by the LICENSE file in this repository.

package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/target/flottbot/core"
	"github.com/target/flottbot/models"
	"github.com/target/flottbot/version"
)

func main() {
	var (
		// version flags
		ver = flag.Bool("version", false, "print version information")
		v   = flag.Bool("v", false, "print version information")

		// bot vars
		rules      = make(map[string]models.Rule)
		hitRule    = make(chan models.Rule, 1)
		inputMsgs  = make(chan models.Message, 1)
		outputMsgs = make(chan models.Message, 1)
	)

	// parse the flagS
	flag.Parse()

	// check to see if the version was requested
	if *v || *ver {
		fmt.Println(version.String())
		os.Exit(0)
	}

	// set some early defaults for the logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	log.Logger = log.Output(os.Stdout).With().Logger()

	// Configure the bot to the core framework
	bot := models.NewBot()
	core.Configure(bot)

	// Populate the global rules map
	core.Rules(&rules, bot)

	// Initialize and run Prometheus metrics logging
	go core.Prommetric("init", bot)

	// Create the wait group for handling concurrent runs (see further down)
	// Add 3 to the wait group so the three separate processes run concurrently
	// - process 1: core.Remotes - reads messages
	// - process 2: core.Matcher - processes messages
	// - process 3: core.Outputs - sends out messages
	var wg sync.WaitGroup

	wg.Add(3)

	go core.Remotes(inputMsgs, rules, bot)
	go core.Matcher(inputMsgs, outputMsgs, rules, hitRule, bot)
	go core.Outputs(outputMsgs, hitRule, bot)

	defer wg.Done()

	// This will run the bot indefinitely because the wait group will
	// attempt to wait for the above never-ending go routines.
	// Since said go routines run forever, they will never finish
	// and so this program will wait, or essentially run, forever until
	// terminated
	wg.Wait()
}
