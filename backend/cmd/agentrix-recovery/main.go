package main

import (
	"agentrix/backend/internal/recovery"
	"agentrix/backend/internal/validation"
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"time"
)

func main() {
	var bots, replays recovery.Roots
	var checkBot, arbiter string
	flag.StringVar(&checkBot, "check-bot", "", "check actual admission of one recovered bot (no DB mutation)")
	flag.StringVar(&arbiter, "arbiter", "", "trusted arbiter binary for --check-bot")
	flag.StringVar(&bots.Original, "original-bots", "", "original absolute bots root")
	flag.StringVar(&bots.Staged, "staged-bots", "", "verified staging bots root")
	flag.StringVar(&bots.Final, "final-bots", "", "new final bots root")
	flag.StringVar(&replays.Original, "original-replays", "", "original absolute replay root")
	flag.StringVar(&replays.Staged, "staged-replays", "", "verified staging replay root")
	flag.StringVar(&replays.Final, "final-replays", "", "new final replay root")
	flag.Parse()
	if checkBot != "" {
		manifest, err := validation.InspectBotDirectory(checkBot)
		if err == nil {
			err = validation.TestBotProtocol(checkBot, manifest, arbiter)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "recovered bot admission failed:", err)
			os.Exit(1)
		}
		fmt.Println("Recovered bot passed isolated admission.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	// libpq PG* environment; never expose credentials in process arguments.
	conn, err := pgx.Connect(ctx, "")
	if err == nil {
		defer conn.Close(ctx)
		err = recovery.VerifyAndRemap(ctx, conn, bots, replays)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "recovery verification failed:", err)
		os.Exit(1)
	}
	fmt.Println("Artifact verification and transactional path remapping completed.")
}
