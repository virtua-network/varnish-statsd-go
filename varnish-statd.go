// Virtua Varnish statistics emitter for statsd/graphite
// 
// Worflow :
//  - get varnish 4.1 statistics via HTTP call
//  - parse json
//  - send UDP statsd frames

package main

import (
    "encoding/json"
    "flag"
    "log"
    "net/http"
    "os"
    "time"

    // GOlang statsd implementation
    "gopkg.in/alexcesaro/statsd.v1"
)

// configuration options are defined here. Please see the provided config.json
// sample.
type Configuration struct {
    VarnishUrl        string
    StatsdAddr        string
    StatsdPrefix      string
    SleepPeriod       time.Duration
}

// Varnish JSON structure
type VarnishStats struct {
    Uptime float64 `json:"uptime_sec"`
    AbsoluteHitRate float64 `json:"absolute_hitrate"`
    AverageHitRate float64 `json:"avg_hitrate"`
    AverageLoad float64 `json:"avg_load"`
    //ServerId string `json:"server_id"`
    MainCacheHit map[string]interface{} `json:"MAIN.cache_hit"`
    MainCacheMiss map[string]interface{} `json:"MAIN.cache_miss"`
}

// this is the core function : get Varnish statistics via HTTP and send them to
// remote statsd.
func StatsEmitter(configuration Configuration) {
    // HTTP call
    cHttp := &http.Client{}
    req, err := http.NewRequest("GET", configuration.VarnishUrl, nil)
    resp, err := cHttp.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    // for debugging : dump HTTP body
    //io.Copy(os.Stdout, resp.Body)

    stats := VarnishStats{}
    // JSON decoder (parsing HTTP body)
    json.NewDecoder(resp.Body).Decode(&stats)

    // open connection with statsd
    cStatsd, err := statsd.New(configuration.StatsdAddr,
                               statsd.WithPrefix(configuration.StatsdPrefix))
    if err != nil {
        panic(err)
    }

    // compute records for statsd
    varnishCacheHit := stats.MainCacheHit["value"].(float64)
    varnishCacheMiss := stats.MainCacheMiss["value"].(float64)

    // DEBUG snippet
    //fmt.Printf("uptime:%f|g\n",
    //            stats.Uptime)
    //fmt.Printf("cache_hit:%f|g\n",
    //            stats.MainCacheHit["value"])

    // send to statsd
    cStatsd.Gauge("uptime", int(stats.Uptime))
    cStatsd.Gauge("absolute_hitrate", int(stats.AverageHitRate))
    cStatsd.Gauge("avg_hitrate", int(stats.AverageHitRate))
    cStatsd.Gauge("avg_load", int(stats.AverageLoad))
    cStatsd.Gauge("cache_hit", int(varnishCacheHit))
    cStatsd.Gauge("cache_miss", int(varnishCacheMiss))

    // as we have send all the stats values, close connection with server
    cStatsd.Close()
}

func main() {
    log.Print("Starting varnish-statsd...")

    // opts handling
    configFlag := flag.String("config", "config.json", "configuration file location")
    flag.Parse()

    // reading config file (mandatory)
    file, errno := os.Open(*configFlag)
    if errno != nil {
        log.Fatal("configuration file is not found nor readable")
    }

    // configuration file processing (JSON)
    decoder := json.NewDecoder(file)

    // close file on exit and check for its returned error
    defer func() {
        if err := file.Close(); err != nil {
            panic(err)
        }
    }()

    configuration := Configuration{}
    err := decoder.Decode(&configuration)
    if err != nil {
        panic(err)
    }

    // main loop : this will run infinite
    for {
        StatsEmitter(configuration)
        timeDuration := configuration.SleepPeriod * time.Second
        time.Sleep(timeDuration)
    }
}
