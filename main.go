package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Mode     string `json:"mode"`      // "server" or "client"
	Edge     string `json:"edge"`      // "left", "right", "top", "bottom"
	ServerIP string `json:"server_ip"` // "auto" or IP
	Port     int    `json:"port"`
}

func configFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "flowkvm.json"
	}
	return filepath.Join(filepath.Dir(exe), "flowkvm.json")
}

func loadConfig() (*Config, bool) {
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return nil, false
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, false
	}
	if cfg.Mode != "server" && cfg.Mode != "client" {
		return nil, false
	}
	if cfg.Port <= 0 {
		cfg.Port = DefaultTCPPort
	}
	if cfg.Edge == "" {
		cfg.Edge = "left"
	}
	return &cfg, true
}

func saveConfig(cfg *Config) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile(configFilePath(), data, 0644)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n[KRİTİK HATA / PANIC]: %v\n", r)
			fmt.Println("Pencerenin kapanmaması için bekleniyor (Çıkmak için Ctrl+C)...")
			for {
				time.Sleep(10 * time.Second)
			}
		}
	}()

	EnableDPIAwareness()

	modeFlag := flag.String("mode", "", "Calisma modu: 'server' (Ana PC) veya 'client' (Diger PC)")
	edgeFlag := flag.String("edge", "left", "Diger ekranin yonu: 'left' (Sol), 'right' (Sag), 'top' (Ust), 'bottom' (Alt)")
	ipFlag := flag.String("ip", "auto", "Hedef IP adresi (varsayilan: 'auto' - otomatik ag kesfi)")
	portFlag := flag.Int("port", DefaultTCPPort, "TCP Port numarasi")
	resetFlag := flag.Bool("reset", false, "Kayitli ayarlari sifirla ve menuyu ac")
	flag.Parse()

	cfg := &Config{
		Mode:     strings.ToLower(strings.TrimSpace(*modeFlag)),
		Edge:     strings.ToLower(strings.TrimSpace(*edgeFlag)),
		ServerIP: strings.TrimSpace(*ipFlag),
		Port:     *portFlag,
	}

	// If built as client-only binary (FlowKVM-DigerPC.exe), always run directly in client mode!
	exeName := strings.ToLower(filepath.Base(os.Args[0]))
	if DefaultBuildMode == "client" || strings.Contains(exeName, "diger") || strings.Contains(exeName, "client") {
		cfg.Mode = "client"
	} else if strings.Contains(exeName, "ana") || strings.Contains(exeName, "server") {
		cfg.Mode = "server"
	}

	if cfg.Mode == "" && !*resetFlag {
		if saved, ok := loadConfig(); ok {
			cfg = saved
			fmt.Printf("[i] Kayıtlı ayarlar yüklendi (%s).\n\n", configFilePath())
		}
	}

	reader := bufio.NewReader(os.Stdin)

	if cfg.Mode != "server" && cfg.Mode != "client" {
		fmt.Println("================================================================")
		fmt.Println("         FlowKVM - Kesintisiz Klavye & Fare Paylaşımı           ")
		fmt.Println("================================================================")
		fmt.Println("Bu bilgisayarı hangi rolde kullanmak istiyorsunuz?")
		fmt.Println("  [1] ANA BİLGİSAYAR (Server) - Fiziksel fare ve klavyenin takılı olduğu PC")
		fmt.Println("  [2] DİĞER BİLGİSAYAR (Client) - Kontrol edilecek 2. PC")
		fmt.Print("\nSeçiminiz (1 veya 2): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice == "2" {
			cfg.Mode = "client"
			cfg.ServerIP = "auto"
		} else {
			cfg.Mode = "server"
			cfg.Edge = "left"
		}
		saveConfig(cfg)
		fmt.Println()
	}

	// Never exit or close the window! If Start() ever returns an error, retry automatically.
	for {
		var err error
		if cfg.Mode == "server" {
			srv := NewServer(cfg.Port, cfg.Edge, cfg.ServerIP)
			err = srv.Start()
		} else {
			cli := NewClient(cfg.ServerIP, cfg.Port)
			err = cli.Start()
		}

		if err != nil {
			fmt.Printf("\n[UYARI] %v - 2 saniye içinde yeniden deneniyor...\n", err)
		}
		time.Sleep(2 * time.Second)
	}
}
