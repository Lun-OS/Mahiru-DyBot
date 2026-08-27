package onebot

// 系统信息 API

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// SystemInfo 系统信息
type SystemInfo struct {
	OS           string `json:"os"`
	Platform     string `json:"platform"`
	Arch         string `json:"arch"`
	KernelVersion string `json:"kernel_version"`
	Hostname     string `json:"hostname"`
	GoVersion    string `json:"go_version"`
	Uptime       uint64 `json:"uptime"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model       string  `json:"model"`
	Cores       int     `json:"cores"`
	UsagePercent float64 `json:"usage_percent"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	BytesSent     uint64 `json:"bytes_sent"`
	BytesRecv     uint64 `json:"bytes_recv"`
	UploadSpeed   uint64 `json:"upload_speed"`
	DownloadSpeed uint64 `json:"download_speed"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	MemoryMB   float64 `json:"memory_mb"`
	MemoryPercent float64 `json:"memory_percent"`
	CPUPercent float64 `json:"cpu_percent"`
	StartTime  string  `json:"start_time"`
	Uptime     string  `json:"uptime"`
}

// SystemResponse 系统信息响应
type SystemResponse struct {
	System  SystemInfo  `json:"system"`
	CPU     CPUInfo     `json:"cpu"`
	Memory  MemoryInfo  `json:"memory"`
	Disk    DiskInfo    `json:"disk"`
	Network NetworkInfo `json:"network"`
	Process ProcessInfo `json:"process"`
}

var lastNetBytes = struct {
	sent     uint64
	recv     uint64
	lastTime time.Time
}{}

// handleSystemInfo GET /api/webui/system/info
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	resp := SystemResponse{}

	// System info
	hostInfo, _ := host.Info()
	resp.System = SystemInfo{
		OS:            runtime.GOOS,
		Platform:      hostInfo.Platform,
		Arch:          runtime.GOARCH,
		KernelVersion: hostInfo.KernelVersion,
		Hostname:      hostInfo.Hostname,
		GoVersion:     runtime.Version(),
		Uptime:        hostInfo.Uptime,
	}

	// CPU info
	cpuInfos, _ := cpu.Info()
	cores := runtime.NumCPU()
	model := "Unknown"
	if len(cpuInfos) > 0 {
		model = cpuInfos[0].ModelName
	}
	cpuPercents, _ := cpu.Percent(time.Second, false)
	usage := 0.0
	if len(cpuPercents) > 0 {
		usage = cpuPercents[0]
	}
	resp.CPU = CPUInfo{
		Model:        model,
		Cores:        cores,
		UsagePercent: usage,
	}

	// Memory info
	vmStat, _ := mem.VirtualMemory()
	if vmStat != nil {
		resp.Memory = MemoryInfo{
			Total:        vmStat.Total,
			Used:         vmStat.Used,
			Free:         vmStat.Free,
			UsagePercent: vmStat.UsedPercent,
		}
	}

	// Disk info
	diskStat, _ := disk.Usage("/")
	if diskStat != nil {
		resp.Disk = DiskInfo{
			Total:        diskStat.Total,
			Used:         diskStat.Used,
			Free:         diskStat.Free,
			UsagePercent: diskStat.UsedPercent,
		}
	}

	// Network info
	ioStats, _ := net.IOCounters(false)
	if len(ioStats) > 0 {
		now := time.Now()
		sent := ioStats[0].BytesSent
		recv := ioStats[0].BytesRecv

		uploadSpeed := uint64(0)
		downloadSpeed := uint64(0)

		if !lastNetBytes.lastTime.IsZero() {
			elapsed := now.Sub(lastNetBytes.lastTime).Seconds()
			if elapsed > 0 {
				if sent >= lastNetBytes.sent {
					uploadSpeed = uint64(float64(sent-lastNetBytes.sent) / elapsed)
				}
				if recv >= lastNetBytes.recv {
					downloadSpeed = uint64(float64(recv-lastNetBytes.recv) / elapsed)
				}
			}
		}
		lastNetBytes.sent = sent
		lastNetBytes.recv = recv
		lastNetBytes.lastTime = now

		resp.Network = NetworkInfo{
			BytesSent:     sent,
			BytesRecv:     recv,
			UploadSpeed:   uploadSpeed,
			DownloadSpeed: downloadSpeed,
		}
	}

	// Process info
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err == nil {
		resp.Process = ProcessInfo{
			PID:   int32(os.Getpid()),
			Name:  func() string { n, _ := proc.Name(); return n }(),
			MemoryMB: func() float64 {
				m, _ := proc.MemoryInfo()
				if m != nil {
					return float64(m.RSS) / 1024 / 1024
				}
				return 0
			}(),
			MemoryPercent: func() float64 { p, _ := proc.MemoryPercent(); return float64(p) }(),
			CPUPercent:    func() float64 { p, _ := proc.CPUPercent(); return p }(),
			StartTime: func() string {
				t, _ := proc.CreateTime()
				if t > 0 {
					return time.UnixMilli(t).Format("2006-01-02 15:04:05")
				}
				return ""
			}(),
			Uptime: func() string {
				t, _ := proc.CreateTime()
				if t > 0 {
					d := time.Since(time.UnixMilli(t))
					h := int(d.Hours())
					m := int(d.Minutes()) % 60
					return fmt.Sprintf("%dh %dm", h, m)
				}
				return ""
			}(),
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}
