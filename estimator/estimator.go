package estimator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

type Estimator struct {
	Samples  int64
	RPC      string
	Threads  int64
	Statmode StatMode
}
type StatMode int

const (
	STATMODE_MEAN StatMode = iota
	STATMODE_MEDIAN
)

var hClient = retryablehttp.NewClient()

func init() {
	hClient.Logger = log.New(ioutil.Discard, "", log.LstdFlags)
}

type Number interface {
	constraints.Float | constraints.Integer
}

func median[T Number](data []T) float64 {
	dataCopy := make([]T, len(data))
	copy(dataCopy, data)

	slices.Sort(dataCopy)

	var median float64
	l := len(dataCopy)
	if l == 0 {
		return 0
	} else if l%2 == 0 {
		median = float64((dataCopy[l/2-1] + dataCopy[l/2]) / 2.0)
	} else {
		median = float64(dataCopy[l/2])
	}

	return median
}
func (e *Estimator) getCurHeight() (int64, error) {

	resp, err := hClient.Get(fmt.Sprintf("%s/status", e.RPC))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("unexpected /status response status code %d", resp.StatusCode)
	}

	var statusResp StatusResponse
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&statusResp)
	if err != nil {
		return 0, err
	}
	ourInt, err := strconv.ParseInt(statusResp.Result.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return 0, err
	}

	return ourInt, nil
}
func (e *Estimator) getBlockTime(height int64) (time.Time, error) {
	resp, err := hClient.Get(fmt.Sprintf("%s/block?height=%d", e.RPC, height))
	if err != nil {
		return time.Now(), err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return time.Now(), fmt.Errorf("unexpected /block response status code %d", resp.StatusCode)
	}

	var blockResp BlockResponse
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&blockResp)
	if err != nil {
		return time.Now(), err
	}

	return blockResp.Result.Block.Header.Time, nil
}

func (e *Estimator) getAvgBlockDurationNanos() (time.Duration, error) {
	curHeight, err := e.getCurHeight()
	if err != nil {
		return 0, err
	}

	var statmodeDesc string
	var avgDur time.Duration
	wp := workerpool.New(int(e.Threads))
	mut := sync.Mutex{}
	makeQuerier := func(sink []time.Time, i int, blockHeight int64, bar *progressbar.ProgressBar) func() {
		return func() {
			blockTime, err := e.getBlockTime(blockHeight)
			if err == nil {
				mut.Lock()
				sink[i] = blockTime
				mut.Unlock()
			}
			if bar != nil {
				bar.Add(1)
			}
		}
	}
	switch e.Statmode {
	case STATMODE_MEAN:
		// Estimate from just the two endpoints.
		statmodeDesc = "Mean"
		resultSamples := make([]time.Time, 2)
		for i, height := range []int64{curHeight - e.Samples, curHeight} {
			wp.Submit(makeQuerier(resultSamples, i, height, nil))
		}
		wp.StopWait()
		totalNanoseconds := resultSamples[1].UnixNano() - resultSamples[0].UnixNano()
		avgDur = time.Duration(totalNanoseconds / e.Samples)
	case STATMODE_MEDIAN:
		// Take the necessary samples, starting at current height and moving backwards.
		statmodeDesc = "Median"
		resultSamples := make([]time.Time, e.Samples+1)
		bar := progressbar.Default(e.Samples)
		for i := int64(0); i <= e.Samples; i++ {
			wp.Submit(makeQuerier(resultSamples, int(i), curHeight-i, bar))
		}
		wp.StopWait()
		bar.Finish()
		fmt.Println("")

		// Calculate deltas and take the median.
		deltas := make([]time.Duration, e.Samples)
		for i := 1; i < len(resultSamples); i++ {
			diff := resultSamples[i-1].UnixNano() - resultSamples[i].UnixNano()
			deltas[i-1] = time.Duration(diff)
		}
		avgDur = time.Duration(median(deltas))
	default:
		return 0, fmt.Errorf("invalid statmode")
	}

	fmt.Printf("%s Block Time: %fs (%d samples)\n", statmodeDesc, avgDur.Seconds(), e.Samples)
	return avgDur, nil
}

func (e *Estimator) CalcBlock(date time.Time) (int64, error) {
	if date.Unix() <= time.Now().Unix() {
		return 0, fmt.Errorf("date to estimate must be in the future")
	}
	avgTime, _ := e.getAvgBlockDurationNanos()

	curHeight2, err := e.getCurHeight()
	if err != nil {
		return 0, err
	}

	estimatedBlocks := time.Until(date).Nanoseconds() / avgTime.Nanoseconds()

	return curHeight2 + estimatedBlocks, nil
}
func (e *Estimator) CalcDate(height int64) (time.Time, error) {
	curHeight, err := e.getCurHeight()
	if err != nil {
		return time.Now(), err
	}
	if height <= curHeight {
		return time.Now(), fmt.Errorf("height to estimate must be greater than current height %d", curHeight)
	}
	avgTime, _ := e.getAvgBlockDurationNanos()

	curHeight2, err := e.getCurHeight()
	if err != nil {
		return time.Now(), err
	}
	gapTime := time.Duration(int64(avgTime) * (height - curHeight2))

	return time.Now().Add(gapTime), nil
}
