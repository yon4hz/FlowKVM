# FlowKVM

**FlowKVM**, aynı ağdaki (veya Telefon USB Tethering / NAT arkasındaki) iki Windows bilgisayarı **tek bir fiziksel fare ve klavye** ile kesintisiz kontrol etmenizi sağlayan, **Go (Golang)** ile geliştirilmiş ultra düşük gecikmeli bir yazılım KVM (Keyboard, Video, Mouse) ve dosya paylaşım aracıdır.

Deskflow / Barrier / Synergy gibi araçlarda sıkça yaşanan **Timeout (Zaman Aşımı)**, **USB Tethering / NAT bağlantı engeli** ve **DirectInput oyunlarında (örn. Metin2) fare/klavye çalışmama** sorunlarını tamamen çözmek üzere tasarlanmıştır.

---

## ✨ Özellikler

- **Sıfır Timeout & Otomatik Yeniden Bağlanma:** `TCP_NODELAY`, aktif `Ping/Pong` heartbeat ve kopma anında milisaniyeler içinde otomatik yeniden bağlanma.
- **USB Tethering / NAT Desteği (Ters Yönlü Bağlantı):** Ana bilgisayarınız telefona USB kablosuyla (USB Tethering - örn. `10.94.0.x`) bağlı, diğer bilgisayarınız ise ev Wi-Fi ağında (`192.168.0.x` / `192.168.1.x`) olsa bile, alt ağı otomatik tarayıp el sıkışma (`Handshake`) doğrulamasıyla doğrudan bağlanır.
- **125Hz Akıllı Fare Paketleme (Coalescing):** 1000Hz oyuncu farelerinin ağ tamponunu şişirmesini önlemek için fare hareketlerini 8ms (125Hz) aralıklarla atomik olarak birleştirir.
- **Oyun & DirectInput Uyumluluğu (Metin2 vb.):** Hem mutlak (`MOUSEEVENTF_ABSOLUTE`) hem bağıl (`MOUSEEVENTF_MOVE` 3D kamera döndürme) fare hareketlerini ve donanım klavye `ScanCode` (`MapVirtualKeyW`) sinyallerini destekler.
- **Failsafe (Fare Hapsini Önleme):** Fare diğer ekrandayken ağ bağlantısı kesilirse imleci anında ana bilgisayara geri çağırır.
- **Sınırsız Boyutlu Dosya & Klasör Kopyala-Yapıştır (`CF_HDROP`):** Fare/klavye kanalından (`24800`) tamamen bağımsız ikinci bir TCP kanalı (`24802`) üzerinden diskten diske `64 KB` akış (streaming) ile boyut sınırı olmadan dosya ve klasörleri `Ctrl + C` / `Ctrl + V` ile kopyalar.
- **Çift Yönlü Metin Panosu (`CF_UNICODETEXT`):** `32 MB`'a kadar Türkçe karakter destekli anlık metin kopyala-yapıştır.
- **Per-Monitor High-DPI V2 Desteği:** %100, %125, %150 gibi farklı ekran ölçeklendirmelerine sahip bilgisayarlar arasında 1:1 piksel doğruluğu.

---

## 🚀 Hızlı Kurulum ve Kullanım

### 1. Hazır `.exe` Dosyaları İle Kullanım
- **Ana Bilgisayar (Fiziksel Fare ve Klavyenin Takılı Olduğu PC):**
  - Görünür konsol ile çalıştırmak için: `FlowKVM-AnaPC.exe`
  - Arka planda tamamen gizli (CMD penceresiz) çalıştırmak için: `FlowKVM-AnaPC-Gizli.exe`
  - Windows açılışında otomatik başlaması için: `OtoBaslat-Ekle-AnaPC.bat`
- **Diğer Bilgisayar (Kontrol Edilecek 2. PC):**
  - Görünür konsol ile çalıştırmak için: `FlowKVM-DigerPC.exe`
  - Arka planda tamamen gizli (CMD penceresiz) çalıştırmak için: `FlowKVM-DigerPC-Gizli.exe`
  - Windows açılışında Yönetici yetkisiyle otomatik başlaması için: `OtoBaslat-Ekle-DigerPC.bat`

### 2. Kaynak Koddan Derleme
Sisteminizde [Go 1.22+](https://go.dev/dl/) kuruluysa `Derle-Build.bat` dosyasına çift tıklayarak veya terminalden aşağıdaki komutlarla tüm `.exe` dosyalarını üretebilirsiniz:

```powershell
# Testleri çalıştır
go test -v .

# Ana Bilgisayar (Server) sürümleri
go build -o FlowKVM-AnaPC.exe .
go build -ldflags="-H=windowsgui" -o FlowKVM-AnaPC-Gizli.exe .

# Diğer Bilgisayar (Client - Sıfır Hook API içeren temiz istemci) sürümleri
go build -tags clientonly -o FlowKVM-DigerPC.exe .
go build -tags clientonly -ldflags="-H=windowsgui" -o FlowKVM-DigerPC-Gizli.exe .
```

---

## ⌨️ Kısayollar ve Kontroller

| İşlem | Kısayol / Hareket |
| :--- | :--- |
| **Diğer Bilgisayara Geçiş** | Fareyi ekranın sol kenarına götürün *(veya `Ctrl + Alt + S`)* |
| **Ana Bilgisayara Dönüş** | Diğer ekranda fareyi sağ kenara götürün *(veya `Ctrl + Alt + S`)* |
| **Acil Geri Çağırma (Failsafe)** | `Ctrl + Alt + Q` veya `Pause / Break` |
| **Metin / Dosya Kopyalama** | Her iki bilgisayarda standart `Ctrl + C` ve `Ctrl + V` |
| **Arka Plan İşlemini Kapatma** | `FlowKVM-Durdur.bat` |

---

## 📂 Proje Yapısı

- `main.go`: Komut satırı parametreleri, yapılandırma (`flowkvm.json`) ve otomatik yeniden başlatma döngüsü.
- `protocol.go`: İkili (binary) TCP paket protokolü (`MsgHandshake`, `MsgMouseMove`, `MsgMouseButton`, `MsgKeyEvent`, `MsgClipboardText`, `MsgPing`/`MsgPong`).
- `server.go`: Ana bilgisayar düşük seviyeli fare/klavye kancaları (`WH_MOUSE_LL`, `WH_KEYBOARD_LL`), görünmez imleç kilitleyici pencere, alt ağ tarayıcı ve 125Hz fare paketleyici.
- `client.go`: Diğer bilgisayar girdi sentezleyici, çift yönlü bağlantı dinleyicisi ve pano izleyicisi.
- `filetransfer.go`: Port `24802` üzerinden sınırsız boyutlu dosya ve klasör akış (streaming) yöneticisi.
- `winapi.go` & `winapi_server.go`: Saf Go (`syscall`) ile Windows `user32.dll`, `kernel32.dll` ve `shell32.dll` API bağlamaları (CGO gerektirmez).
