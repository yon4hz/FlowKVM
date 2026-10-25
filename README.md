<div align="center">

# ⚡ FlowKVM

**Ultra-Low Latency Software KVM (Mouse, Keyboard, Clipboard & Unlimited File Sharing) for Windows**  
*Zero-Timeout Architecture • USB Tethering / NAT Traversal • DirectInput Game Support • Pure Go*

[![Language: Go](https://img.shields.io/badge/Language-Go%201.22%2B-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Platform: Windows](https://img.shields.io/badge/Platform-Windows%2010%2F11-0078D6?style=flat-square&logo=windows)](https://github.com/yon4hz/FlowKVM)
[![Zero Dependencies](https://img.shields.io/badge/Dependencies-Zero%20(Stdlib%20%2B%20Win32)-success?style=flat-square)](https://github.com/yon4hz/FlowKVM)

### 🌍 [**🇬🇧 English**](#-english) &nbsp;|&nbsp; [**🇹🇷 Türkçe**](#-türkçe)

</div>

---

<a name="-english"></a>
## 🇬🇧 English

**FlowKVM** is a lightweight, high-performance software KVM (Keyboard, Video, Mouse) and file-sharing tool written in pure **Go (Golang)** for Windows. It lets you seamlessly control two computers using a **single physical mouse and keyboard** simply by moving your cursor across the screen edge.

It was specifically engineered to eliminate the most common pain points found in traditional tools like Deskflow, Barrier, and Synergy: **connection timeouts**, **USB Tethering / NAT isolation**, **1000Hz gaming mouse lag**, and **incompatibility with DirectInput / Administrator games (such as Metin2)**.

### ✨ Key Features

- **🛡️ Zero-Timeout & Instant Auto-Reconnect:** Powered by `TCP_NODELAY`, active `Ping/Pong` heartbeats, and instant background reconnection within milliseconds—preventing frozen cursors or dead sessions.
- **📱 USB Tethering & NAT Traversal (Reverse-Connect):** Works even when your Main PC is connected via **Phone USB Tethering** (e.g., `10.94.0.x` behind NAT) and your Second PC is on **Home Wi-Fi** (`192.168.0.x` / `192.168.1.x`). FlowKVM automatically scans upstream subnets, filters out NAT hairpin IPs, verifies peers via cryptographic-style `Handshake`, and establishes the connection in the permitted NAT direction.
- **🖱️ 125Hz Smart Mouse Coalescing:** Aggregates 1000Hz gaming mouse deltas into smooth 8ms (125Hz) atomic frames, reducing network packet overhead by **90%** while keeping clicks and keystrokes on a zero-delay priority path.
- **🎮 DirectInput & Game Compatibility (Metin2, etc.):** Supports both absolute (`MOUSEEVENTF_ABSOLUTE`) and relative (`MOUSEEVENTF_MOVE` for 3D right-click camera rotation) mouse injection, plus hardware `ScanCode` (`MapVirtualKeyW`) translation for DirectInput8 games.
- **🔒 Automatic Failsafe Recall:** If the network drops while your cursor is on the secondary PC, FlowKVM immediately recalls your mouse and keyboard back to your primary screen so your cursor is never trapped.
- **📂 Unlimited File & Folder Copy-Paste (`CF_HDROP`):** Uses a **dedicated second TCP channel (Port `24802`)** completely isolated from the real-time Mouse/Keyboard stream (Port `24800`). Copy (`Ctrl+C`) and paste (`Ctrl+V`) single files, multiple files, or entire directory trees of **any size (500 MB, 5 GB, 50 GB+)** with `64 KB` disk-to-disk streaming and near-zero RAM usage.
- **📋 Bidirectional Text Clipboard (`CF_UNICODETEXT`):** Instant Unicode text copy-paste up to **32 MB** between computers.
- **🖥️ Per-Monitor High-DPI V2 Aware:** Accurate 1:1 physical pixel mapping across monitors with different scaling factors (100%, 125%, 150%).

---

### 🚀 Quick Start (English)

#### 1. Using Prebuilt Executables
- **On Your Main PC (Server - where physical mouse & keyboard are plugged in):**
  - Visible Console Mode: Run `FlowKVM-AnaPC.exe`
  - Silent Background Mode (No CMD Window): Run `FlowKVM-AnaPC-Gizli.exe`
  - Auto-Start on Windows Boot: Run `OtoBaslat-Ekle-AnaPC.bat` once.
- **On Your Second PC (Client - the computer to be controlled):**
  - Visible Console Mode: Run `FlowKVM-DigerPC.exe` *(Right-click -> **Run as administrator** if controlling elevated games like Metin2)*
  - Silent Background Mode (No CMD Window): Run `FlowKVM-DigerPC-Gizli.exe`
  - Auto-Start on Windows Boot (with Admin rights): Run `OtoBaslat-Ekle-DigerPC.bat` once.

#### 2. Building from Source
Requires [Go 1.22+](https://go.dev/dl/) on Windows (zero external library dependencies):

```powershell
# Run unit tests
go test -v .

# Build Main PC (Server) executables
go build -o FlowKVM-AnaPC.exe .
go build -ldflags="-H=windowsgui" -o FlowKVM-AnaPC-Gizli.exe .

# Build Second PC (Client - clean build with zero Hook APIs) executables
go build -tags clientonly -o FlowKVM-DigerPC.exe .
go build -tags clientonly -ldflags="-H=windowsgui" -o FlowKVM-DigerPC-Gizli.exe .
```

#### ⌨️ Controls & Hotkeys

| Action | Trigger / Hotkey |
| :--- | :--- |
| **Switch to Second PC** | Move cursor to the **left edge** of Main PC *(or press `Ctrl + Alt + S`)* |
| **Return to Main PC** | Move cursor to the **right edge** of Second PC *(or press `Ctrl + Alt + S`)* |
| **Emergency Recall (Failsafe)** | `Ctrl + Alt + Q` or `Pause / Break` |
| **Copy-Paste Text / Files** | Standard `Ctrl + C` on one PC, then `Ctrl + V` on the other PC |
| **Stop Background Process** | Run `FlowKVM-Durdur.bat` |

---
---

<a name="-türkçe"></a>
## 🇹🇷 Türkçe

**FlowKVM**, aynı ağdaki (veya Telefon USB Tethering / NAT arkasındaki) iki Windows bilgisayarı **tek bir fiziksel fare ve klavye** ile kesintisiz kontrol etmenizi sağlayan, saf **Go (Golang)** ile geliştirilmiş ultra düşük gecikmeli bir yazılım KVM (Keyboard, Video, Mouse) ve dosya paylaşım aracıdır.

Deskflow, Barrier ve Synergy gibi araçlarda sıkça yaşanan **Timeout (Zaman Aşımı)**, **Telefon USB Tethering / NAT bağlantı engeli**, **1000Hz oyuncu faresi tıkanması** ve **DirectInput oyunlarında (örn. Metin2) fare/klavye çalışmama** sorunlarını tamamen çözmek üzere tasarlanmıştır.

### ✨ Özellikler

- **🛡️ Sıfır Timeout & Otomatik Yeniden Bağlanma:** `TCP_NODELAY`, aktif `Ping/Pong` heartbeat ve kopma anında milisaniyeler içinde otomatik yeniden bağlanma.
- **📱 USB Tethering / NAT Desteği (Ters Yönlü Bağlantı):** Ana bilgisayarınız telefona USB kablosuyla (USB Tethering - örn. `10.94.0.x`) bağlı, diğer bilgisayarınız ise ev Wi-Fi ağında (`192.168.0.x` / `192.168.1.x`) olsa bile, alt ağı otomatik tarayıp el sıkışma (`Handshake`) doğrulamasıyla doğrudan bağlanır.
- **🖱️ 125Hz Akıllı Fare Paketleme (Coalescing):** 1000Hz oyuncu farelerinin ağ tamponunu şişirmesini önlemek için fare hareketlerini 8ms (125Hz) aralıklarla atomik olarak birleştirir; ağ trafiğini **%90 azaltır**.
- **🎮 Oyun & DirectInput Uyumluluğu (Metin2 vb.):** Hem mutlak (`MOUSEEVENTF_ABSOLUTE`) hem bağıl (`MOUSEEVENTF_MOVE` sağ tık 3D kamera döndürme) fare hareketlerini ve donanım klavye `ScanCode` (`MapVirtualKeyW`) sinyallerini destekler.
- **🔒 Failsafe (Fare Hapsini Önleme):** Fare diğer ekrandayken ağ bağlantısı kesilirse imleci anında ana bilgisayara geri çağırır.
- **📂 Sınırsız Boyutlu Dosya & Klasör Kopyala-Yapıştır (`CF_HDROP`):** Fare/klavye kanalından (`24800`) tamamen bağımsız ikinci bir TCP kanalı (`24802`) üzerinden diskten diske `64 KB` akış (streaming) ile boyut sınırı olmadan (500 MB, 5 GB, 50 GB+) dosya ve klasörleri `Ctrl + C` / `Ctrl + V` ile kopyalar.
- **📋 Çift Yönlü Metin Panosu (`CF_UNICODETEXT`):** `32 MB`'a kadar Türkçe karakter destekli anlık metin kopyala-yapıştır.
- **🖥️ Per-Monitor High-DPI V2 Desteği:** %100, %125, %150 gibi farklı ekran ölçeklendirmelerine sahip bilgisayarlar arasında 1:1 piksel doğruluğu.

---

### 🚀 Hızlı Kurulum ve Kullanım (Türkçe)

#### 1. Hazır `.exe` Dosyaları İle Kullanım
- **Ana Bilgisayar (Fiziksel Fare ve Klavyenin Takılı Olduğu PC):**
  - Pencereli (Konsol) Mod: `FlowKVM-AnaPC.exe`
  - Arka Planda Gizli (CMD Penceresiz) Mod: `FlowKVM-AnaPC-Gizli.exe`
  - Bilgisayar Açılışında Otomatik Başlatma: `OtoBaslat-Ekle-AnaPC.bat`
- **Diğer Bilgisayar (Kontrol Edilecek 2. PC):**
  - Pencereli (Konsol) Mod: `FlowKVM-DigerPC.exe` *(Metin2 gibi oyunları kontrol etmek için Sağ Tık -> **Yönetici olarak çalıştır**)*
  - Arka Planda Gizli (CMD Penceresiz) Mod: `FlowKVM-DigerPC-Gizli.exe`
  - Bilgisayar Açılışında Yönetici Yetkisiyle Otomatik Başlatma: `OtoBaslat-Ekle-DigerPC.bat`

#### 2. Kaynak Koddan Derleme
Sisteminizde [Go 1.22+](https://go.dev/dl/) kuruluysa `Derle-Build.bat` dosyasına çift tıklayarak veya terminalden derleyebilirsiniz:

```powershell
# Birim testlerini çalıştır
go test -v .

# Ana Bilgisayar (Server) sürümleri
go build -o FlowKVM-AnaPC.exe .
go build -ldflags="-H=windowsgui" -o FlowKVM-AnaPC-Gizli.exe .

# Diğer Bilgisayar (Client - Sıfır Hook API içeren temiz istemci) sürümleri
go build -tags clientonly -o FlowKVM-DigerPC.exe .
go build -tags clientonly -ldflags="-H=windowsgui" -o FlowKVM-DigerPC-Gizli.exe .
```

#### ⌨️ Kısayollar ve Kontroller

| İşlem | Kısayol / Hareket |
| :--- | :--- |
| **Diğer Bilgisayara Geçiş** | Fareyi ekranın **sol kenarına** götürün *(veya `Ctrl + Alt + S`)* |
| **Ana Bilgisayara Dönüş** | Diğer ekranda fareyi **sağ kenara** götürün *(veya `Ctrl + Alt + S`)* |
| **Acil Geri Çağırma (Failsafe)** | `Ctrl + Alt + Q` veya `Pause / Break` |
| **Metin / Dosya Kopyalama** | Her iki bilgisayarda standart `Ctrl + C` ve `Ctrl + V` |
| **Arka Plan İşlemini Kapatma** | `FlowKVM-Durdur.bat` |

---

## 📂 Architecture / Proje Yapısı

| File / Dosya | Description (EN) | Açıklama (TR) |
| :--- | :--- | :--- |
| `main.go` | CLI flags, config (`flowkvm.json`) & auto-recovery loop | Komut satırı parametreleri, ayarlar ve otomatik kurtarma döngüsü |
| `protocol.go` | Binary TCP protocol (`TCP_NODELAY`, `Ping/Pong`, `Handshake`) | Düşük gecikmeli ikili (binary) TCP paket protokolü |
| `server.go` | Low-level hooks (`WH_MOUSE_LL`, `WH_KEYBOARD_LL`), subnet scanner & 125Hz coalescer | Düşük seviyeli kancalar, görünmez imleç kilidi, alt ağ tarayıcı ve 125Hz fare paketleyici |
| `client.go` | Input synthesizer, reverse-connect listener & clipboard watcher | Girdi sentezleyici, çift yönlü bağlantı dinleyicisi ve pano izleyicisi |
| `filetransfer.go` | Dedicated port `24802` streaming manager for unlimited file/folder copy-paste | Port `24802` üzerinden sınırsız boyutlu dosya/klasör akış (streaming) yöneticisi |
| `winapi.go` | Pure Go (`syscall`) bindings for `user32.dll`, `kernel32.dll`, `shell32.dll` | Saf Go ile Windows API (`SendInput`, `CF_HDROP`, `CF_UNICODETEXT`) bağlamaları |
| `winapi_server.go` | Server-only hook bindings (isolated via `!clientonly` build tag) | Yalnızca sunucu derlemesine dahil edilen kanca API'leri |
