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
	Lang     string `json:"lang"`      // "tr" or "en"
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
			fmt.Printf("\n[CRITICAL ERROR / KRİTİK HATA]: %v\n", r)
			for {
				time.Sleep(10 * time.Second)
			}
		}
	}()

	EnableDPIAwareness()

	langFlag := flag.String("lang", "auto", "Language / Dil: 'auto', 'en' (English), 'tr' (Turkce)")
	modeFlag := flag.String("mode", "", "Mode / Calisma modu: 'server' (Main PC) or 'client' (Second PC)")
	edgeFlag := flag.String("edge", "left", "Screen edge / Ekran yonu: 'left', 'right', 'top', 'bottom'")
	ipFlag := flag.String("ip", "auto", "Target IP / Hedef IP ('auto' for automatic LAN/NAT discovery)")
	portFlag := flag.Int("port", DefaultTCPPort, "TCP Port")
	resetFlag := flag.Bool("reset", false, "Reset saved config / Kayitli ayarlari sifirla")
	flag.Parse()

	SetLanguage(*langFlag)

	cfg := &Config{
		Lang:     currentLang,
		Mode:     strings.ToLower(strings.TrimSpace(*modeFlag)),
		Edge:     strings.ToLower(strings.TrimSpace(*edgeFlag)),
		ServerIP: strings.TrimSpace(*ipFlag),
		Port:     *portFlag,
	}

	exeName := strings.ToLower(filepath.Base(os.Args[0]))
	if DefaultBuildMode == "client" || strings.Contains(exeName, "diger") || strings.Contains(exeName, "client") {
		cfg.Mode = "client"
	} else if strings.Contains(exeName, "ana") || strings.Contains(exeName, "server") {
		cfg.Mode = "server"
	}

	if cfg.Mode == "" && !*resetFlag {
		if saved, ok := loadConfig(); ok {
			cfg = saved
			if *langFlag == "auto" && saved.Lang != "" {
				SetLanguage(saved.Lang)
			}
			fmt.Printf("[i] %s (%s)\n\n",
				T("Kayıtlı ayarlar yüklendi", "Loaded saved configuration"),
				configFilePath())
		}
	}

	reader := bufio.NewReader(os.Stdin)

	if cfg.Mode != "server" && cfg.Mode != "client" {
		fmt.Println("================================================================")
		fmt.Println("   FlowKVM - Seamless Software KVM & File Sharing (EN / TR)     ")
		fmt.Println("================================================================")
		fmt.Println("Select Language / Dil Seçiniz:")
		fmt.Println("  [1] English")
		fmt.Println("  [2] Türkçe")
		fmt.Print("\nChoice / Seçim (Enter = Auto): ")
		langChoice, _ := reader.ReadString('\n')
		langChoice = strings.TrimSpace(langChoice)
		if langChoice != "" {
			SetLanguage(langChoice)
		}
		cfg.Lang = currentLang

		fmt.Println("\n----------------------------------------------------------------")
		fmt.Println(T(
			"Bu bilgisayarı hangi rolde kullanmak istiyorsunuz?",
			"Which role should this computer run as?",
		))
		fmt.Println(T(
			"  [1] ANA BİLGİSAYAR (Server) - Fiziksel fare ve klavyenin takılı olduğu PC",
			"  [1] MAIN PC (Server) - Computer with physical mouse and keyboard attached",
		))
		fmt.Println(T(
			"  [2] DİĞER BİLGİSAYAR (Client) - Kontrol edilecek 2. PC",
			"  [2] SECOND PC (Client) - Computer to be controlled remotely",
		))
		fmt.Print(T("\nSeçiminiz (1 veya 2): ", "\nYour choice (1 or 2): "))
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice == "2" {
			cfg.Mode = "client"
			cfg.ServerIP = "auto"
		} else {
			cfg.Mode = "server"
			fmt.Println(T(
				"\nDiğer bilgisayar ekranınızın hangi tarafında duruyor?",
				"\nWhich side of your main screen is the second computer located on?",
			))
			fmt.Println(T("  [1] SOLDA (Varsayılan)", "  [1] LEFT (Default)"))
			fmt.Println(T("  [2] SAĞDA", "  [2] RIGHT"))
			fmt.Println(T("  [3] ÜSTTE", "  [3] TOP"))
			fmt.Println(T("  [4] ALTTA", "  [4] BOTTOM"))
			fmt.Print(T("\nSeçiminiz (Enter = 1): ", "\nYour choice (Enter = 1): "))
			edgeChoice, _ := reader.ReadString('\n')
			edgeChoice = strings.TrimSpace(edgeChoice)
			switch edgeChoice {
			case "2":
				cfg.Edge = "right"
			case "3":
				cfg.Edge = "top"
			case "4":
				cfg.Edge = "bottom"
			default:
				cfg.Edge = "left"
			}
		}
		saveConfig(cfg)
		fmt.Println()
	}

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
			fmt.Printf("\n[!] %v - %s\n", err, T("2 saniye içinde yeniden deneniyor...", "Retrying in 2 seconds..."))
		}
		time.Sleep(2 * time.Second)
	}
}
