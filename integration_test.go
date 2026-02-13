package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/providers/bluesky"
	"github.com/chcolte/fediverse-archive-bot-go/providers/mastodon"
	"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
	"github.com/chcolte/fediverse-archive-bot-go/providers/nostr"
)

const (
	// テスト用のタイムアウト（各プロバイダーのメッセージ受信待機時間）
	testReceiveTimeout = 15 * time.Second
)

// =========================================================================
// Misskey Integration Test
// =========================================================================

func TestIntegration_Misskey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	serverURL := "misskey.io"
	timeline := "local"
	downloadDir := t.TempDir()

	t.Logf("Misskey: connecting to %s (timeline=%s, dir=%s)", serverURL, timeline, downloadDir)

	provider := misskey.NewMisskeyProvider(serverURL, timeline, downloadDir)

	// 1. Connect
	if err := provider.Connect(); err != nil {
		t.Fatalf("Misskey: Connect() failed: %v", err)
	}
	defer provider.Close()
	t.Log("Misskey: connected successfully")

	// 2. ConnectChannel
	if err := provider.ConnectChannel(); err != nil {
		t.Fatalf("Misskey: ConnectChannel() failed: %v", err)
	}
	t.Log("Misskey: channel connected")

	// 3. ReceiveMessages (goroutine)
	output := make(chan models.DownloadItem, 100)
	errCh := make(chan error, 1)
	receivedCount := 0

	go func() {
		errCh <- provider.ReceiveMessages(output)
	}()

	// 4. タイムアウトまでメッセージを受信
	timer := time.NewTimer(testReceiveTimeout)
	defer timer.Stop()

loop_misskey:
	for {
		select {
		case item := <-output:
			receivedCount++
			t.Logf("Misskey: received download item #%d: %s", receivedCount, truncateURL(item.URL))
		case <-timer.C:
			t.Logf("Misskey: timeout reached after %v, received %d items", testReceiveTimeout, receivedCount)
			break loop_misskey
		case err := <-errCh:
			if err != nil {
				t.Logf("Misskey: ReceiveMessages returned error: %v", err)
			}
			break loop_misskey
		}
	}

	// 5. Close
	provider.Close()

	// 6. ファイル保存の検証
	verifyFilesExist(t, "Misskey", downloadDir)
}

// =========================================================================
// Mastodon Integration Test
// =========================================================================

func TestIntegration_Mastodon(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	serverURL := "mstdn.jp"
	timeline := "global"
	downloadDir := t.TempDir()

	t.Logf("Mastodon: connecting to %s (timeline=%s, dir=%s)", serverURL, timeline, downloadDir)

	provider := mastodon.NewMastodonProvider(serverURL, timeline, downloadDir)

	// 1. Connect
	if err := provider.Connect(); err != nil {
		t.Fatalf("Mastodon: Connect() failed: %v", err)
	}
	defer provider.Close()
	t.Log("Mastodon: connected successfully")

	// 2. ConnectChannel
	if err := provider.ConnectChannel(); err != nil {
		t.Fatalf("Mastodon: ConnectChannel() failed: %v", err)
	}
	t.Log("Mastodon: channel connected")

	// 3. ReceiveMessages (goroutine)
	output := make(chan models.DownloadItem, 100)
	errCh := make(chan error, 1)
	receivedCount := 0

	go func() {
		errCh <- provider.ReceiveMessages(output)
	}()

	// 4. タイムアウトまでメッセージを受信
	timer := time.NewTimer(testReceiveTimeout)
	defer timer.Stop()

loop_mastodon:
	for {
		select {
		case item := <-output:
			receivedCount++
			t.Logf("Mastodon: received download item #%d: %s", receivedCount, truncateURL(item.URL))
		case <-timer.C:
			t.Logf("Mastodon: timeout reached after %v, received %d items", testReceiveTimeout, receivedCount)
			break loop_mastodon
		case err := <-errCh:
			if err != nil {
				t.Logf("Mastodon: ReceiveMessages returned error: %v", err)
			}
			break loop_mastodon
		}
	}

	// 5. Close
	provider.Close()

	// 6. ファイル保存の検証
	verifyFilesExist(t, "Mastodon", downloadDir)
}

// =========================================================================
// Bluesky Integration Test
// =========================================================================

func TestIntegration_Bluesky(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Bluesky Firehose endpoint
	serverURL := "wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos"
	downloadDir := t.TempDir()

	t.Logf("Bluesky: connecting to firehose (dir=%s)", downloadDir)

	provider := bluesky.NewBlueskyProvider(serverURL, downloadDir)

	// 1. Connect
	if err := provider.Connect(); err != nil {
		t.Fatalf("Bluesky: Connect() failed: %v", err)
	}
	defer provider.Close()
	t.Log("Bluesky: connected successfully")

	// 2. ConnectChannel (Blueskyでは不要だがインターフェース互換のため呼ぶ)
	if err := provider.ConnectChannel(); err != nil {
		t.Fatalf("Bluesky: ConnectChannel() failed: %v", err)
	}

	// 3. ReceiveMessages (goroutine)
	output := make(chan models.DownloadItem, 100)
	errCh := make(chan error, 1)
	receivedCount := 0

	go func() {
		errCh <- provider.ReceiveMessages(output)
	}()

	// 4. タイムアウトまでメッセージを受信
	// Bluesky Firehoseは大量のメッセージが流れるため短めでOK
	timer := time.NewTimer(testReceiveTimeout)
	defer timer.Stop()

loop_bluesky:
	for {
		select {
		case item := <-output:
			receivedCount++
			if receivedCount <= 5 {
				t.Logf("Bluesky: received download item #%d: %s", receivedCount, truncateURL(item.URL))
			}
			if receivedCount == 5 {
				t.Logf("Bluesky: (suppressing further item logs...)")
			}
		case <-timer.C:
			t.Logf("Bluesky: timeout reached after %v, received %d items", testReceiveTimeout, receivedCount)
			break loop_bluesky
		case err := <-errCh:
			if err != nil {
				t.Logf("Bluesky: ReceiveMessages returned error: %v", err)
			}
			break loop_bluesky
		}
	}

	// 5. Close
	provider.Close()

	// 6. ファイル保存の検証
	verifyFilesExist(t, "Bluesky", downloadDir)

	// Blueskyの場合はCBORファイルも確認
	verifyCBORFilesExist(t, downloadDir)
}

// =========================================================================
// Nostr Integration Test
// =========================================================================

func TestIntegration_Nostr(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	serverURL := "wss://relay.damus.io"
	downloadDir := t.TempDir()

	t.Logf("Nostr: connecting to %s (dir=%s)", serverURL, downloadDir)

	provider := nostr.NewNostrProvider(serverURL, downloadDir)

	// 1. Connect
	if err := provider.Connect(); err != nil {
		t.Fatalf("Nostr: Connect() failed: %v", err)
	}
	defer provider.Close()
	t.Log("Nostr: connected successfully")

	// 2. ConnectChannel (REQを送信)
	if err := provider.ConnectChannel(); err != nil {
		t.Fatalf("Nostr: ConnectChannel() failed: %v", err)
	}
	t.Log("Nostr: channel connected (REQ sent)")

	// 3. ReceiveMessages (goroutine)
	output := make(chan models.DownloadItem, 100)
	errCh := make(chan error, 1)
	receivedCount := 0

	go func() {
		errCh <- provider.ReceiveMessages(output)
	}()

	// 4. タイムアウトまでメッセージを受信
	timer := time.NewTimer(testReceiveTimeout)
	defer timer.Stop()

loop_nostr:
	for {
		select {
		case item := <-output:
			receivedCount++
			if receivedCount <= 5 {
				t.Logf("Nostr: received download item #%d: %s", receivedCount, truncateURL(item.URL))
			}
			if receivedCount == 5 {
				t.Logf("Nostr: (suppressing further item logs...)")
			}
		case <-timer.C:
			t.Logf("Nostr: timeout reached after %v, received %d items", testReceiveTimeout, receivedCount)
			break loop_nostr
		case err := <-errCh:
			if err != nil {
				t.Logf("Nostr: ReceiveMessages returned error: %v", err)
			}
			break loop_nostr
		}
	}

	// 5. Close
	provider.Close()

	// 6. ファイル保存の検証
	verifyFilesExist(t, "Nostr", downloadDir)
}

// =========================================================================
// 全プロバイダー一括テスト
// =========================================================================

func TestIntegration_AllProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("Misskey", func(t *testing.T) {
		TestIntegration_Misskey(t)
	})
	t.Run("Mastodon", func(t *testing.T) {
		TestIntegration_Mastodon(t)
	})
	t.Run("Bluesky", func(t *testing.T) {
		TestIntegration_Bluesky(t)
	})
	t.Run("Nostr", func(t *testing.T) {
		TestIntegration_Nostr(t)
	})
}

// =========================================================================
// Helper functions
// =========================================================================

// verifyFilesExist はダウンロードディレクトリ内にファイルが作成されているか検証する
func verifyFilesExist(t *testing.T, providerName, downloadDir string) {
	t.Helper()

	fileCount := 0
	totalSize := int64(0)

	err := filepath.Walk(downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
			totalSize += info.Size()
			relPath, _ := filepath.Rel(downloadDir, path)
			t.Logf("%s: found file: %s (size=%d bytes)", providerName, relPath, info.Size())
		}
		return nil
	})

	if err != nil {
		t.Errorf("%s: failed to walk download directory: %v", providerName, err)
		return
	}

	if fileCount == 0 {
		t.Errorf("%s: FAIL - no files were created in download directory", providerName)
	} else {
		t.Logf("%s: OK - %d files created, total size: %d bytes", providerName, fileCount, totalSize)
	}

	if totalSize == 0 {
		t.Errorf("%s: FAIL - total file size is 0", providerName)
	}
}

// verifyCBORFilesExist はBluesky用のCBORファイルが作成されているか検証する
func verifyCBORFilesExist(t *testing.T, downloadDir string) {
	t.Helper()

	cborCount := 0
	err := filepath.Walk(downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".cbor" {
			cborCount++
		}
		return nil
	})

	if err != nil {
		t.Errorf("Bluesky: failed to walk for CBOR files: %v", err)
		return
	}

	if cborCount == 0 {
		t.Errorf("Bluesky: FAIL - no CBOR files were created")
	} else {
		t.Logf("Bluesky: OK - %d CBOR files found", cborCount)
	}
}

// truncateURL はURLを表示用に短縮する
func truncateURL(url string) string {
	if len(url) > 80 {
		return fmt.Sprintf("%s...(%d chars)", url[:77], len(url))
	}
	return url
}
