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
		return 0, fmt.Errorf("invalid status %d", resp.StatusCode)
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
		return time.Now(), fmt.Errorf("invalid status %d", resp.StatusCode)
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
	wp := workerpool.New(int(e.Threads))
	mut := sync.Mutex{}
	resultSamples := make([]time.Time, e.Samples+1)

	curHeight, err := e.getCurHeight()
	if err != nil {
		return 0, err
	}
	bar := progressbar.Default(e.Samples)

	for i := int64(0); i <= e.Samples; i++ {
		i := i
		wp.Submit(func() {
			blockToQuery := curHeight - i
			blockHeight, err := e.getBlockTime(blockToQuery)
			if err == nil {
				mut.Lock()
				resultSamples[i] = blockHeight
				mut.Unlock()
			}
			bar.Add(1)
		})
	}
	wp.StopWait()
	bar.Finish()
	fmt.Println("")

	deltas := make([]time.Duration, e.Samples)
	for i := 1; i < len(resultSamples); i++ {
		diff := resultSamples[i-1].UnixNano() - resultSamples[i].UnixNano()
		deltas[i-1] = time.Duration(diff)
	}
	switch e.Statmode {
	case STATMODE_MEAN:
		sumDur := time.Duration(0)
		for i := 0; i < len(deltas); i++ {
			sumDur += deltas[i]
		}
		avgDur := time.Duration(sumDur.Nanoseconds() / int64(len(deltas)))
		fmt.Printf("Mean Block Time: %fs (%d samples)\n", avgDur.Seconds(), e.Samples)
		return avgDur, nil
	case STATMODE_MEDIAN:
		medianBlockTime := time.Duration(median(deltas))
		fmt.Printf("Median Block Time: %fs (%d samples)\n", medianBlockTime.Seconds(), e.Samples)
		return medianBlockTime, nil
	default:
		return 0, fmt.Errorf("invalid statmode")
	}
}

func (e *Estimator) CalcBlock(ttime time.Time) (int64, error) {
	if ttime.Unix() <= time.Now().Unix() {
		return 0, fmt.Errorf("time to estimate is < less than the current time")
	}
	avgTime, _ := e.getAvgBlockDurationNanos()

	curHeight2, err := e.getCurHeight()
	if err != nil {
		return 0, err
	}

	estimatedBlocks := ttime.Sub(time.Now()).Nanoseconds() / avgTime.Nanoseconds()

	return curHeight2 + estimatedBlocks, nil
}
func (e *Estimator) CalcDate(height int64) (time.Time, error) {

	curHeight, err := e.getCurHeight()
	if err != nil {
		return time.Now(), err
	}
	if height <= curHeight {
		return time.Now(), fmt.Errorf("height to estimate (%d) is < current height of %d", height, curHeight)
	}
	avgTime, _ := e.getAvgBlockDurationNanos()

	curHeight2, err := e.getCurHeight()
	if err != nil {
		return time.Now(), err
	}
	gapTime := time.Duration(int64(avgTime) * (height - curHeight2))

	return time.Now().Add(gapTime), nil
}
