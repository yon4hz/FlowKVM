package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	FileMsgBatchStart byte = 1
	FileMsgEntry      byte = 2
	FileMsgBatchEnd   byte = 3
	FileMsgPing       byte = 4
)

type fileItem struct {
	absPath string
	relPath string
	isDir   bool
	size    int64
}

// FileTransferManager manages the dedicated port 24802 TCP stream for unlimited-size file/folder copy-paste
// completely isolated from the real-time Mouse/Keyboard socket.
type FileTransferManager struct {
	port        int
	stagingDir  string
	mu          sync.Mutex
	activeConn  net.Conn
	peerIP      string
	connected   atomic.Bool
	sending     atomic.Bool
	onClipReady func()
}

func NewFileTransferManager(port int, onClipReady func()) *FileTransferManager {
	baseDir, err := os.UserHomeDir()
	if err != nil || baseDir == "" {
		baseDir = "."
	}
	staging := filepath.Join(baseDir, "Downloads", "FlowKVM_Paylasilan")
	return &FileTransferManager{
		port:        port,
		stagingDir:  staging,
		onClipReady: onClipReady,
	}
}

func (ft *FileTransferManager) IsStagingPath(paths []string) bool {
	stageClean := strings.ToLower(filepath.Clean(ft.stagingDir))
	for _, p := range paths {
		cleanP := strings.ToLower(filepath.Clean(p))
		if strings.HasPrefix(cleanP, stageClean) {
			return true
		}
		// Never auto-transfer FlowKVM's own executable when user copies it to install on the other PC
		base := strings.ToLower(filepath.Base(cleanP))
		if strings.HasPrefix(base, "flowkvm") && strings.HasSuffix(base, ".exe") {
			return true
		}
	}
	return false
}

func (ft *FileTransferManager) StartListener() {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", ft.port))
	if err != nil {
		return
	}
	safeGo(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			ft.attachConn(conn)
		}
	})
}

func (ft *FileTransferManager) SetPeerIP(peerIP string, dialNow bool) {
	if peerIP == "" {
		return
	}
	ft.mu.Lock()
	ft.peerIP = peerIP
	ft.mu.Unlock()

	if dialNow {
		safeGo(func() {
			time.Sleep(400 * time.Millisecond)
			for i := 0; i < 3; i++ {
				if ft.connected.Load() {
					return
				}
				conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", peerIP, ft.port), 1500*time.Millisecond)
				if err == nil {
					ft.attachConn(conn)
					return
				}
				time.Sleep(500 * time.Millisecond)
			}
		})
	}
}

func (ft *FileTransferManager) attachConn(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(5 * time.Second)
		_ = tcp.SetReadBuffer(256 * 1024)
		_ = tcp.SetWriteBuffer(256 * 1024)
	}

	ft.mu.Lock()
	if ft.activeConn != nil && ft.connected.Load() {
		ft.mu.Unlock()
		_ = conn.Close()
		return
	}
	ft.activeConn = conn
	ft.connected.Store(true)
	ft.mu.Unlock()

	safeGo(func() {
		ft.readLoop(conn)
	})
}

func (ft *FileTransferManager) ensureConn() net.Conn {
	ft.mu.Lock()
	c := ft.activeConn
	peer := ft.peerIP
	ft.mu.Unlock()
	if c != nil && ft.connected.Load() {
		return c
	}
	if peer == "" {
		return nil
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", peer, ft.port), 2000*time.Millisecond)
	if err != nil {
		return nil
	}
	ft.attachConn(conn)
	return conn
}

func (ft *FileTransferManager) SendFilesAsync(rootPaths []string) {
	if len(rootPaths) == 0 || ft.IsStagingPath(rootPaths) {
		return
	}
	if !ft.sending.CompareAndSwap(false, true) {
		return
	}

	safeGo(func() {
		defer ft.sending.Store(false)
		if err := ft.sendFiles(rootPaths); err != nil {
			fmt.Printf("[!] Dosya aktarım hatası: %v\n", err)
		}
	})
}

func (ft *FileTransferManager) collectItems(rootPaths []string) ([]string, []fileItem, int64) {
	var topNames []string
	var items []fileItem
	var totalBytes int64
	seenTop := make(map[string]bool)

	for _, root := range rootPaths {
		root = filepath.Clean(root)
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		baseName := filepath.Base(root)
		if baseName == "" || baseName == "." || baseName == string(filepath.Separator) || seenTop[baseName] {
			continue
		}
		seenTop[baseName] = true
		topNames = append(topNames, baseName)

		if !info.IsDir() {
			items = append(items, fileItem{
				absPath: root,
				relPath: baseName,
				isDir:   false,
				size:    info.Size(),
			})
			totalBytes += info.Size()
			continue
		}

		parentDir := filepath.Dir(root)
		_ = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, rerr := filepath.Rel(parentDir, path)
			if rerr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if fi.IsDir() {
				items = append(items, fileItem{
					absPath: path,
					relPath: rel,
					isDir:   true,
					size:    0,
				})
			} else if fi.Mode().IsRegular() {
				items = append(items, fileItem{
					absPath: path,
					relPath: rel,
					isDir:   false,
					size:    fi.Size(),
				})
				totalBytes += fi.Size()
			}
			return nil
		})
	}
	return topNames, items, totalBytes
}

func (ft *FileTransferManager) sendFiles(rootPaths []string) error {
	topNames, items, totalBytes := ft.collectItems(rootPaths)
	if len(items) == 0 || len(topNames) == 0 {
		return nil
	}

	conn := ft.ensureConn()
	if conn == nil {
		return fmt.Errorf("dosya kanalı bağlı değil")
	}

	fmt.Printf("\n[Dosya Gönderiliyor] %d öğe (Toplam: %.2f MB) arka planda aktarılıyor...\n",
		len(topNames), float64(totalBytes)/(1024*1024))

	w := bufio.NewWriterSize(conn, 128*1024)
	var hdr [16]byte

	if err := w.WriteByte(FileMsgBatchStart); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(topNames)))
	binary.LittleEndian.PutUint64(hdr[4:12], uint64(totalBytes))
	if _, err := w.Write(hdr[:12]); err != nil {
		return err
	}
	for _, name := range topNames {
		nb := []byte(name)
		binary.LittleEndian.PutUint16(hdr[0:2], uint16(len(nb)))
		if _, err := w.Write(hdr[:2]); err != nil {
			return err
		}
		if _, err := w.Write(nb); err != nil {
			return err
		}
	}

	copyBuf := make([]byte, 64*1024)
	var sentBytes int64
	lastLog := time.Now()

	for _, item := range items {
		if err := w.WriteByte(FileMsgEntry); err != nil {
			return err
		}
		if item.isDir {
			hdr[0] = 1
		} else {
			hdr[0] = 0
		}
		relBytes := []byte(item.relPath)
		binary.LittleEndian.PutUint16(hdr[1:3], uint16(len(relBytes)))
		binary.LittleEndian.PutUint64(hdr[3:11], uint64(item.size))
		if _, err := w.Write(hdr[:11]); err != nil {
			return err
		}
		if _, err := w.Write(relBytes); err != nil {
			return err
		}

		if !item.isDir && item.size > 0 {
			f, err := os.Open(item.absPath)
			if err != nil {
				rem := item.size
				zeroBuf := make([]byte, 32*1024)
				for rem > 0 {
					chunk := int64(len(zeroBuf))
					if rem < chunk {
						chunk = rem
					}
					if _, werr := w.Write(zeroBuf[:chunk]); werr != nil {
						return werr
					}
					rem -= chunk
				}
				continue
			}

			rem := item.size
			for rem > 0 {
				toRead := int64(len(copyBuf))
				if rem < toRead {
					toRead = rem
				}
				n, rerr := f.Read(copyBuf[:toRead])
				if n > 0 {
					if _, werr := w.Write(copyBuf[:n]); werr != nil {
						_ = f.Close()
						return werr
					}
					rem -= int64(n)
					sentBytes += int64(n)
				}
				if rerr != nil {
					if rem > 0 {
						zeros := make([]byte, 4096)
						for rem > 0 {
							c := int64(len(zeros))
							if rem < c {
								c = rem
							}
							if _, werr := w.Write(zeros[:c]); werr != nil {
								_ = f.Close()
								return werr
							}
							rem -= c
						}
					}
					break
				}
				if time.Since(lastLog) > 1*time.Second && totalBytes > 0 {
					lastLog = time.Now()
					pct := float64(sentBytes) * 100 / float64(totalBytes)
					fmt.Printf("  -> Gönderiliyor: %.1f%% (%.1f MB / %.1f MB)\n",
						pct, float64(sentBytes)/(1024*1024), float64(totalBytes)/(1024*1024))
				}
			}
			_ = f.Close()
		}
	}

	if err := w.WriteByte(FileMsgBatchEnd); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("[✓] Dosya Gönderimi Tamamlandı (%s)!\n", strings.Join(topNames, ", "))
	return nil
}

func (ft *FileTransferManager) readLoop(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		ft.mu.Lock()
		if ft.activeConn == conn {
			ft.activeConn = nil
			ft.connected.Store(false)
		}
		ft.mu.Unlock()
	}()

	r := bufio.NewReaderSize(conn, 128*1024)
	var hdr [16]byte
	var topPaths []string
	var totalBytes int64
	var recvBytes int64
	lastLog := time.Now()

	for {
		msgType, err := r.ReadByte()
		if err != nil {
			return
		}

		switch msgType {
		case FileMsgPing:
			// Keepalive

		case FileMsgBatchStart:
			if _, err := io.ReadFull(r, hdr[:12]); err != nil {
				return
			}
			topCount := binary.LittleEndian.Uint32(hdr[0:4])
			totalBytes = int64(binary.LittleEndian.Uint64(hdr[4:12]))
			recvBytes = 0
			topPaths = nil

			_ = os.MkdirAll(ft.stagingDir, 0755)

			for i := uint32(0); i < topCount; i++ {
				if _, err := io.ReadFull(r, hdr[:2]); err != nil {
					return
				}
				nameLen := binary.LittleEndian.Uint16(hdr[0:2])
				nb := make([]byte, nameLen)
				if _, err := io.ReadFull(r, nb); err != nil {
					return
				}
				safeName := filepath.Base(string(nb))
				topPaths = append(topPaths, filepath.Join(ft.stagingDir, safeName))
			}
			fmt.Printf("\n[Dosya Alınıyor] %d öğe (%.2f MB) arka planda indiriliyor...\n",
				len(topPaths), float64(totalBytes)/(1024*1024))

		case FileMsgEntry:
			if _, err := io.ReadFull(r, hdr[:11]); err != nil {
				return
			}
			isDir := hdr[0] != 0
			relLen := binary.LittleEndian.Uint16(hdr[1:3])
			fSize := int64(binary.LittleEndian.Uint64(hdr[3:11]))

			relBuf := make([]byte, relLen)
			if _, err := io.ReadFull(r, relBuf); err != nil {
				return
			}
			relPath := filepath.FromSlash(string(relBuf))
			cleanRel := filepath.Clean(relPath)
			if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
				cleanRel = filepath.Base(cleanRel)
			}
			destPath := filepath.Join(ft.stagingDir, cleanRel)

			if isDir {
				_ = os.MkdirAll(destPath, 0755)
				continue
			}

			_ = os.MkdirAll(filepath.Dir(destPath), 0755)
			outFile, err := os.Create(destPath)
			if err != nil {
				_, _ = io.CopyN(io.Discard, r, fSize)
				continue
			}

			rem := fSize
			copyBuf := make([]byte, 64*1024)
			for rem > 0 {
				chunk := int64(len(copyBuf))
				if rem < chunk {
					chunk = rem
				}
				n, rerr := io.ReadFull(r, copyBuf[:chunk])
				if n > 0 {
					_, _ = outFile.Write(copyBuf[:n])
					rem -= int64(n)
					recvBytes += int64(n)
				}
				if rerr != nil {
					_ = outFile.Close()
					return
				}
				if time.Since(lastLog) > 1*time.Second && totalBytes > 0 {
					lastLog = time.Now()
					pct := float64(recvBytes) * 100 / float64(totalBytes)
					fmt.Printf("  <- Alınıyor: %.1f%% (%.1f MB / %.1f MB)\n",
						pct, float64(recvBytes)/(1024*1024), float64(totalBytes)/(1024*1024))
				}
			}
			_ = outFile.Close()

		case FileMsgBatchEnd:
			if len(topPaths) > 0 {
				if WriteClipboardFiles(topPaths) {
					if ft.onClipReady != nil {
						ft.onClipReady()
					}
					fmt.Printf("[✓] Dosya Panoya Hazır (%s)! Şimdi Ctrl+V ile yapıştırabilirsiniz.\n", ft.stagingDir)
				}
			}
		}
	}
}
