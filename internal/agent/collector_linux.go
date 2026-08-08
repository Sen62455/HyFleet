//go:build linux

package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyfleet/hyfleet/internal/protocol"
	"golang.org/x/sys/unix"
)

type linuxCollector struct {
	mu          sync.Mutex
	previousCPU [2]uint64
	previousNet networkSample
	facts       HostFacts
}

func NewCollector() Collector {
	return &linuxCollector{facts: readLinuxFacts()}
}

func (collector *linuxCollector) Facts() HostFacts {
	return collector.facts
}

func (collector *linuxCollector) Sample(_ context.Context) (protocol.HostMetrics, error) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	metrics := protocol.HostMetrics{}
	totalCPU, idleCPU, err := readCPU()
	if err != nil {
		return metrics, err
	}
	if collector.previousCPU[0] > 0 && totalCPU > collector.previousCPU[0] {
		totalDelta := totalCPU - collector.previousCPU[0]
		idleDelta := idleCPU - collector.previousCPU[1]
		metrics.CPUPercent = float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	}
	collector.previousCPU = [2]uint64{totalCPU, idleCPU}
	metrics.MemoryTotalBytes, metrics.MemoryUsedBytes, err = readMemory()
	if err != nil {
		return metrics, err
	}
	metrics.UptimeSeconds, err = readUptime()
	if err != nil {
		return metrics, err
	}
	metrics.Load1, metrics.Load5, metrics.Load15, err = readLoad()
	if err != nil {
		return metrics, err
	}
	var disk unix.Statfs_t
	if err := unix.Statfs("/", &disk); err != nil {
		return metrics, fmt.Errorf("read root filesystem: %w", err)
	}
	metrics.DiskTotalBytes = int64(disk.Blocks) * int64(disk.Bsize)
	metrics.DiskUsedBytes = int64(disk.Blocks-disk.Bavail) * int64(disk.Bsize)
	network, err := readNetwork()
	if err != nil {
		return metrics, err
	}
	if !collector.previousNet.at.IsZero() {
		seconds := network.at.Sub(collector.previousNet.at).Seconds()
		if seconds > 0 && network.rxBytes >= collector.previousNet.rxBytes && network.txBytes >= collector.previousNet.txBytes {
			metrics.NetworkRXBPS = int64(float64(network.rxBytes-collector.previousNet.rxBytes) * 8 / seconds)
			metrics.NetworkTXBPS = int64(float64(network.txBytes-collector.previousNet.txBytes) * 8 / seconds)
		}
	}
	collector.previousNet = network
	return metrics, nil
}

func (collector *linuxCollector) ServiceRunning(ctx context.Context, unit string) bool {
	if unit == "" || strings.ContainsAny(unit, " /\\\t\r\n") {
		return false
	}
	command := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit)
	return command.Run() == nil
}

func readLinuxFacts() HostFacts {
	facts := HostFacts{OS: "linux", Architecture: runtime.GOARCH}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return facts
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			values[key] = strings.Trim(value, `"`)
		}
	}
	if values["ID"] != "" {
		facts.OS = values["ID"]
	}
	facts.OSVersion = values["VERSION_ID"]
	return facts
}

func readCPU() (uint64, uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, 0, fmt.Errorf("read /proc/stat: %w", scanner.Err())
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 8 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("unexpected /proc/stat cpu line")
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("parse /proc/stat: %w", err)
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	return total, values[3] + values[4], nil
}

func readMemory() (total int64, used int64, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	var totalKiB, availableKiB int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalKiB = value
		case "MemAvailable":
			availableKiB = value
		}
	}
	if totalKiB == 0 || availableKiB > totalKiB {
		return 0, 0, fmt.Errorf("unexpected /proc/meminfo values")
	}
	return totalKiB * 1024, (totalKiB - availableKiB) * 1024, nil
}

func readUptime() (int64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read /proc/uptime: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected /proc/uptime value")
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse /proc/uptime: %w", err)
	}
	return int64(value), nil
}

func readLoad() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read /proc/loadavg: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg value")
	}
	values := make([]float64, 3)
	for index := range values {
		values[index], err = strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("parse /proc/loadavg: %w", err)
		}
	}
	return values[0], values[1], values[2], nil
}

func readNetwork() (networkSample, error) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return networkSample{}, fmt.Errorf("read /proc/net/dev: %w", err)
	}
	sample := networkSample{at: timeNow()}
	for _, line := range strings.Split(string(data), "\n") {
		namePart, countersPart, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(namePart) == "lo" {
			continue
		}
		fields := strings.Fields(countersPart)
		if len(fields) < 9 {
			continue
		}
		rx, rxErr := strconv.ParseInt(fields[0], 10, 64)
		tx, txErr := strconv.ParseInt(fields[8], 10, 64)
		if rxErr == nil && txErr == nil {
			sample.rxBytes += rx
			sample.txBytes += tx
		}
	}
	return sample, nil
}

var timeNow = time.Now
