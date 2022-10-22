package main

import (
	"estimator/estimator"
	"flag"
	"fmt"
	"github.com/araddon/dateparse"
	"strings"
	"time"
)

func main() {

	fHeight := flag.Int64("height", 0, "specific height to estimate the time of")
	fDate := flag.String("date", "", "specific time to estimate height at, e.g. Mon Jan 2 15:04:05 MST 2006")
	fSamples := flag.Int64("samples", 100, "number of samples to take")
	fRpc := flag.String("rpc", "https://main.rpc.agoric.net:443", "the rpc endpoint to sample from")
	fVotingPeriod := flag.Duration("votingPeriod", 24*3*time.Hour, "the voting period of the destination chain")
	fThreads := flag.Int64("threads", 6, "number of threads to use")
	fStatmode := flag.String("statmode", "mean", "statmode: mean or median")
	fTimezones := flag.String("timezones", "Local,UTC,Asia/Tokyo,Australia/NSW,Asia/Istanbul,US/Pacific,US/Eastern", "comma-separated list of timezones")
	flag.Parse()

	if *fHeight > 0 && *fDate != "" {
		fmt.Println("must specify either height or date estimation")
		return
	}

	if *fSamples <= 0 {
		fmt.Println("samples must be positive")
		return
	}
	statmode := estimator.STATMODE_MEAN
	if *fStatmode == "median" {
		statmode = estimator.STATMODE_MEDIAN
	}
	esti := estimator.Estimator{
		Samples:  *fSamples,
		RPC:      *fRpc,
		Threads:  *fThreads,
		Statmode: statmode,
	}

	if *fHeight > 0 {
		estTime, err := esti.CalcDate(*fHeight)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		if time.Duration(estTime.UnixNano()-time.Now().UnixNano()) < *fVotingPeriod+time.Hour {
			fmt.Println("****** WARNING ******")
			fmt.Println("The Date above is before the voting period ends")
			fmt.Println(time.Duration(estTime.UnixNano() - time.Now().UnixNano()).String())
			fmt.Println("****** WARNING ******")
		}

		fmt.Printf("Estimated time for height %d:\n", *fHeight)
		tzs := strings.Split(*fTimezones, ",")
		ml := 0
		for _, tz := range tzs {
			if len(tz) >= ml {
				ml = len(tz)
			}
		}
		for _, tz := range tzs {
			loc, err := time.LoadLocation(tz)
			if err != nil {
				fmt.Println("unable to find timezone", tz)
			} else {
				fmt.Printf("%"+fmt.Sprintf("%d", ml)+"s: %s\n", tz, estTime.In(loc).Format(time.UnixDate))
			}
		}
	} else if *fDate != "" {
		date, err := dateparse.ParseLocal(*fDate)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		estBlock, err := esti.CalcBlock(date)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if time.Duration(date.UnixNano()-time.Now().UnixNano()) < *fVotingPeriod+time.Hour {
			fmt.Println("****** WARNING ******")
			fmt.Println("The Date above is before the voting period ends")
			fmt.Println(time.Duration(date.UnixNano() - time.Now().UnixNano()).String())
			fmt.Println("****** WARNING ******")
		}

		fmt.Println("Estimated block height at", date.Format(time.UnixDate), "is", estBlock)
	} else {
		fmt.Println("Must provide either a -height or a -date to estimate")
	}
}
