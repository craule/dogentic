package stats

import (
	"log"
	"math"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStats struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	Timestamp   string  `json:"timestamp"`
}

func CollectStats() (*SystemStats, error) {
	// CPU - Total % over 1 second
	percentages, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		log.Printf("Error getting CPU stats: %v", err)
		return nil, err
	}

	cpuVal := 0.0
	if len(percentages) > 0 {
		cpuVal = percentages[0]
	}

	// Memory
	v, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("Error getting Memory stats: %v", err)
		return nil, err
	}

	return &SystemStats{
		CPUUsage:    math.Round(cpuVal*100) / 100,
		MemoryUsage: math.Round(v.UsedPercent*100) / 100,
		Timestamp:   time.Now().Format(time.RFC3339),
	}, nil
}
