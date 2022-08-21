package estimator

import (
	"encoding/json"
	"fmt"
	"github.com/gammazero/workerpool"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/schollz/progressbar/v3"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"time"
)

type Estimator struct {
	Samples int64
	RPC     string
	Threads int64
}

var hClient = retryablehttp.NewClient()

func init() {
	hClient.Logger = log.New(ioutil.Discard, "", log.LstdFlags)
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
	sumDur := time.Duration(0)
	for i := 0; i < len(deltas); i++ {
		sumDur += deltas[i]
	}
	avgDur := time.Duration(sumDur.Nanoseconds() / int64(len(deltas)))
	fmt.Printf("Average Block Time: %fs (%d samples)\n", avgDur.Seconds(), e.Samples)
	return avgDur, nil
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
