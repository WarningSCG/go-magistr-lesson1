package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	serverURL    = "http://srv.msk01.gigacorp.local/_stats"
	pollInterval = time.Second
	maxErrors    = 3
)

type serverStats struct {
	loadAverage      float64
	loadAverageRaw   string
	totalMemory      int64
	usedMemory       int64
	totalDisk        int64
	usedDisk         int64
	networkBandwidth int64
	networkUsage     int64
}

var client = &http.Client{
	Timeout: 5 * time.Second,
}

func main() {
	errorCount := 0

	for {
		stats, err := fetchStats()

		if err != nil {
			errorCount++

			if errorCount >= maxErrors {
				fmt.Println("Unable to fetch server statistic")
				return
			}
		} else {
			errorCount = 0
			checkStats(stats)
		}

		time.Sleep(pollInterval)
	}
}

func fetchStats() (serverStats, error) {
	var stats serverStats

	resp, err := client.Get(serverURL)
	if err != nil {
		return stats, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return stats, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stats, err
	}

	values := strings.Split(strings.TrimSpace(string(body)), ",")

	if len(values) != 7 {
		return stats, fmt.Errorf("invalid statistic format")
	}

	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}

	stats.loadAverageRaw = values[0]

	stats.loadAverage, err = strconv.ParseFloat(values[0], 64)
	if err != nil {
		return serverStats{}, err
	}

	numbers := make([]int64, 6)

	for i := 1; i < 7; i++ {
		numbers[i-1], err = strconv.ParseInt(values[i], 10, 64)
		if err != nil {
			return serverStats{}, err
		}

		if numbers[i-1] < 0 {
			return serverStats{}, fmt.Errorf("negative resource value")
		}
	}

	stats.totalMemory = numbers[0]
	stats.usedMemory = numbers[1]
	stats.totalDisk = numbers[2]
	stats.usedDisk = numbers[3]
	stats.networkBandwidth = numbers[4]
	stats.networkUsage = numbers[5]

	if stats.totalMemory == 0 ||
		stats.totalDisk == 0 ||
		stats.networkBandwidth == 0 {
		return serverStats{}, fmt.Errorf("invalid resource limit")
	}

	return stats, nil
}

func checkStats(stats serverStats) {
	if stats.loadAverage >= 30 {
		fmt.Printf(
			"Load Average is too high: %s\n",
			stats.loadAverageRaw,
		)
	}

	memoryUsage :=
		float64(stats.usedMemory) /
			float64(stats.totalMemory) * 100

	if memoryUsage > 80 {
		fmt.Printf(
			"Memory usage too high: %d%%\n",
			int(memoryUsage),
		)
	}

	diskUsage :=
		float64(stats.usedDisk) /
			float64(stats.totalDisk) * 100

	if diskUsage > 90 {
		freeDisk := stats.totalDisk - stats.usedDisk

		if freeDisk < 0 {
			freeDisk = 0
		}

		freeDiskMB := freeDisk / (1024 * 1024)

		fmt.Printf(
			"Free disk space is too low: %d Mb left\n",
			freeDiskMB,
		)
	}

	networkUsage :=
		float64(stats.networkUsage) /
			float64(stats.networkBandwidth) * 100

	if networkUsage > 90 {
		freeBandwidth :=
			stats.networkBandwidth - stats.networkUsage

		if freeBandwidth < 0 {
			freeBandwidth = 0
		}

		freeBandwidthMbit := freeBandwidth / 1_000_000

		fmt.Printf(
			"Network bandwidth usage high: %d Mbit/s available\n",
			freeBandwidthMbit,
		)
	}
}
