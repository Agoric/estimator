# Block height estimator for Tendermint web RPC endpoints

This utility will help estimate the time for a future blockheight, or the future blockheight for a time.

Usage:

## Height estimation at future date
```shell
go run main.go -date "Aug 21, 2023 1:23PM PDT" -rpc https://main.rpc.agoric.net:443 -samples 500
```
```console
Mean Block Time: 6.243974s (500 samples)
Estimated block height at Mon Aug 21 13:23:00 PDT 2023 is 11234156
```

## Date estimation at future height
```shell
go run main.go -height 11260923 -rpc https://main.rpc.agoric.net:443 -samples 1000
```
```console
Mean Block Time: 6.291125s (1000 samples)
Estimated time for height 11260923:
        Local: Mon Aug 21 06:01:08 PDT 2023
          UTC: Mon Aug 21 13:01:08 UTC 2023
   Asia/Tokyo: Mon Aug 21 22:01:08 JST 2023
Australia/NSW: Mon Aug 21 23:01:08 AEST 2023
Asia/Istanbul: Mon Aug 21 16:01:08 +03 2023
   US/Pacific: Mon Aug 21 06:01:08 PDT 2023
   US/Eastern: Mon Aug 21 09:01:08 EDT 2023
```
