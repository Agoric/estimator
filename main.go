package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"estimator/estimator"

	"github.com/araddon/dateparse"
)

func run() (exitStatus int, err error) {
	fHeight := flag.Int64("height", 0, "specific height to estimate the time of")
	fDate := flag.String("date", "", "specific time to estimate height at, e.g. Mon Jan 2 15:04:05 MST 2006")
	fSamples := flag.Int64("samples", 100, "count of samples to take")
	fRpc := flag.String("rpc", "https://main.rpc.agoric.net:443", "the rpc endpoint to sample from")
	fVotingPeriod := flag.Duration("votingPeriod", 24*3*time.Hour, "the voting period of the destination chain")
	fThreads := flag.Int64("threads", 6, "number of threads to use")
	fStatmode := flag.String("statmode", "mean", `how to estimate from past blocks, "mean" or "median"`)
	fTimezones := flag.String("timezones", "Local,UTC,Asia/Tokyo,Australia/NSW,Asia/Istanbul,US/Pacific,US/Eastern", "comma-separated list of timezones")
	flag.Parse()

	if *fHeight > 0 && *fDate != "" {
		return 64, fmt.Errorf("must not specify both height and date")
	}

	if *fSamples <= 0 {
		return 1, fmt.Errorf("sample count must be positive")
	}
	statmodes := map[string]estimator.StatMode{
		"mean":   estimator.STATMODE_MEAN,
		"median": estimator.STATMODE_MEDIAN,
	}
	statmode, statmodeOk := statmodes[*fStatmode]
	if !statmodeOk {
		return 1, fmt.Errorf("invalid statmode")
	}
	esti := estimator.Estimator{
		Samples:  *fSamples,
		RPC:      *fRpc,
		Threads:  *fThreads,
		Statmode: statmode,
	}

	var timeRemaining time.Duration
	if *fHeight > 0 {
		estTime, err := esti.CalcDate(*fHeight)
		if err != nil {
			return 1, err
		}

		timeRemaining = time.Until(estTime)

		fmt.Printf("Estimated time for height %d:\n", *fHeight)
		tzs := strings.Split(*fTimezones, ",")
		width := 0
		for _, tz := range tzs {
			if len(tz) > width {
				width = len(tz)
			}
		}
		for _, tz := range tzs {
			loc, err := time.LoadLocation(tz)
			if err != nil {
				fmt.Printf("unable to find timezone %q\n", tz)
			} else {
				fmt.Printf("%*s: %s\n", width, tz, estTime.In(loc).Format(time.UnixDate))
			}
		}
	} else if *fDate != "" {
		date, err := dateparse.ParseLocal(*fDate)
		if err != nil {
			return 1, err
		}

		timeRemaining = time.Until(date)

		estBlock, err := esti.CalcBlock(date)
		if err != nil {
			return 1, err
		}

		fmt.Printf("Estimated block height at %s is %v\n", date.Format(time.UnixDate), estBlock)
	} else {
		fmt.Fprintln(os.Stderr, "must specify either a height or a date")
		flag.PrintDefaults()
		return 64, nil
	}

	if timeRemaining < *fVotingPeriod+time.Hour {
		fmt.Println("****** WARNING ******")
		fmt.Println("Insufficient time for voting period:", timeRemaining)
		fmt.Println("****** WARNING ******")
	}

	return 0, nil
}

func main() {
	exitStatus, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if exitStatus != 0 {
		os.Exit(exitStatus)
	}
}
