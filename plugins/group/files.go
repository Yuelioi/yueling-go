package group

import (
	"container/list"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services"
)

// ── Config ───────────────────────────────────────────────────────────────────

const (
	filesMaxDownload     = 3
	filesMaxUpload       = 2
	filesDownloadTimeout = 5 * time.Minute
)

var filesIgnoreExt = map[string]bool{
	"gif": true, "png": true, "jpg": true,
	"jpeg": true, "mp4": true, "webm": true,
}

func filesBackupDir(groupID int64) string {
	return filepath.Join(services.DataPath("groups"), fmt.Sprintf("%d", groupID))
}

// ── Per-group mutex ──────────────────────────────────────────────────────────

var (
	filesLocks   sync.Map // groupID → *sync.Mutex
	filesWorking sync.Map // groupID → bool
)

func filesLock(groupID int64) *sync.Mutex {
	v, _ := filesLocks.LoadOrStore(groupID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func filesTryLock(groupID int64) bool {
	mu := filesLock(groupID)
	if !mu.TryLock() {
		return false
	}
	return true
}

// ── QQ file tree (recursive BFS) ────────────────────────────────────────────

func filesGetAll(api *bot.BotAPI, groupID int64, includeRoot bool) ([]bot.QQFile, []bot.QQFolder, error) {
	root, err := api.GetGroupRootFiles(groupID)
	if err != nil {
		return nil, nil, err
	}

	var allFiles []bot.QQFile
	var allFolders []bot.QQFolder
	allFolders = append(allFolders, root.Folders...)

	if includeRoot {
		allFiles = append(allFiles, root.Files...)
	}

	queue := list.New()
	for i := range root.Folders {
		queue.PushBack(&root.Folders[i])
	}

	for queue.Len() > 0 {
		elem := queue.Front()
		queue.Remove(elem)
		folder := elem.Value.(*bot.QQFolder)

		snap, err := api.GetGroupFilesByFolder(groupID, folder.FolderID)
		if err != nil {
			log.Printf("[files] skip folder %s: %v", folder.FolderName, err)
			continue
		}
		for i := range snap.Files {
			snap.Files[i].FolderName = folder.FolderName
			allFiles = append(allFiles, snap.Files[i])
		}
		allFolders = append(allFolders, snap.Folders...)
		for i := range snap.Folders {
			queue.PushBack(&snap.Folders[i])
		}
	}
	return allFiles, allFolders, nil
}

// ── Local file index ─────────────────────────────────────────────────────────

func filesLocalIndex(dir string) map[string]string {
	index := map[string]string{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		index[rel] = path
		return nil
	})
	return index
}

func filesFileKey(f bot.QQFile) string {
	if f.FolderName != "" {
		return filepath.Join(f.FolderName, f.FileName)
	}
	return f.FileName
}

// ── Download helper ──────────────────────────────────────────────────────────

var filesHTTP = &http.Client{Timeout: filesDownloadTimeout}

func filesDownload(destPath, url string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	resp, err := filesHTTP.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// ── Concurrency helpers ──────────────────────────────────────────────────────

type fileJob func() (string, error) // returns fileKey, error

func runConcurrent(jobs []fileJob, maxWorkers int) (ok []string, fail []string) {
	sem := make(chan struct{}, maxWorkers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j fileJob) {
			defer func() { <-sem; wg.Done() }()
			key, err := j()
			mu.Lock()
			if err != nil {
				fail = append(fail, key)
			} else {
				ok = append(ok, key)
			}
			mu.Unlock()
		}(job)
	}
	wg.Wait()
	return
}

// ── Normalize for query ──────────────────────────────────────────────────────

var reNonWord = regexp.MustCompile(`[^\p{Han}0-9a-z]`)

func filesNormalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') {
			b.WriteRune(r)
		}
	}
	_ = reNonWord
	return b.String()
}

// ── Commands ─────────────────────────────────────────────────────────────────

func RegisterFiles(b *bot.Bot) {
	b.OnFullMatch("群文件备份").Where(bot.AdminOnly{}).Handle(func(ctx *bot.GroupContext) error {
		return withFilesLock(ctx, "群文件备份", func() (string, error) {
			return filesBackup(ctx.BotAPI, ctx.GroupID())
		})
	})

	b.OnFullMatch("群文件恢复").Where(bot.AdminOnly{}).Handle(func(ctx *bot.GroupContext) error {
		return withFilesLock(ctx, "群文件恢复", func() (string, error) {
			return filesRecover(ctx.BotAPI, ctx.GroupID())
		})
	})

	b.OnCommand("群文件清理").Where(bot.AdminOnly{}).Handle(func(ctx *bot.CommandContext) error {
		exts := ctx.Args
		if len(exts) == 0 {
			exts = defaultKeys(filesIgnoreExt)
		}
		return withFilesLock(ctx.GroupContext, "群文件清理", func() (string, error) {
			return filesClear(ctx.BotAPI, ctx.GroupID(), exts)
		})
	})

	b.OnCommand("群文件整理").Where(bot.AdminOnly{}).Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) < 2 {
			return ctx.Reply("用法：群文件整理 <文件夹名> <扩展名1> [扩展名2...]")
		}
		folder := ctx.Args[0]
		exts := ctx.Args[1:]
		return withFilesLock(ctx.GroupContext, "群文件整理", func() (string, error) {
			return filesOrganize(ctx.BotAPI, ctx.GroupID(), folder, exts)
		})
	})

	b.OnFullMatch("本地文件清理").Where(bot.AdminOnly{}).Handle(func(ctx *bot.GroupContext) error {
		dir := filesBackupDir(ctx.GroupID())
		if err := os.RemoveAll(dir); err != nil {
			return ctx.Reply("清理失败：" + err.Error())
		}
		return ctx.Reply("本地备份已清理")
	})

	b.OnCommand("群文件查询").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：群文件查询 <关键词>")
		}
		kw := strings.Join(ctx.Args, " ")
		return withFilesLock(ctx.GroupContext, "群文件查询", func() (string, error) {
			return filesQuery(ctx.BotAPI, ctx.GroupID(), kw)
		})
	})
}

func withFilesLock(ctx *bot.GroupContext, name string, fn func() (string, error)) error {
	if !filesTryLock(ctx.GroupID()) {
		return ctx.Reply("已有任务在执行中，请稍候...")
	}
	defer filesLock(ctx.GroupID()).Unlock()

	ctx.Reply("正在执行 " + name + "，请稍候...")
	start := time.Now()
	msg, err := fn()
	elapsed := time.Since(start).Round(time.Second)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("%s失败：%v", name, err))
	}
	return ctx.Reply(fmt.Sprintf("%s完成（耗时 %s）\n%s", name, elapsed, msg))
}

// ── Service: backup ──────────────────────────────────────────────────────────

func filesBackup(api *bot.BotAPI, groupID int64) (string, error) {
	backupDir := filesBackupDir(groupID)

	files, _, err := filesGetAll(api, groupID, false)
	if err != nil {
		return "", err
	}

	// create local folder structure
	folderSet := map[string]bool{}
	for _, f := range files {
		if f.FolderName != "" {
			folderSet[f.FolderName] = true
		}
	}
	for name := range folderSet {
		os.MkdirAll(filepath.Join(backupDir, name), 0o755)
	}

	local := filesLocalIndex(backupDir)

	var jobs []fileJob
	for _, f := range files {
		f := f
		key := filesFileKey(f)
		if local[key] != "" {
			continue
		}
		jobs = append(jobs, func() (string, error) {
			url, err := api.GetGroupFileURL(groupID, f.FileID, f.BusID)
			if err != nil {
				return key, err
			}
			return key, filesDownload(filepath.Join(backupDir, key), url)
		})
	}

	ok, fail := runConcurrent(jobs, filesMaxDownload)
	return fmt.Sprintf("下载 %d 个，失败 %d 个（共 %d 个）", len(ok), len(fail), len(files)), nil
}

// ── Service: recovery ────────────────────────────────────────────────────────

func filesRecover(api *bot.BotAPI, groupID int64) (string, error) {
	backupDir := filesBackupDir(groupID)
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return "", fmt.Errorf("本地备份不存在")
	}

	local := filesLocalIndex(backupDir)
	if len(local) == 0 {
		return "本地备份为空，无需恢复", nil
	}

	qqFiles, qqFolders, err := filesGetAll(api, groupID, true)
	if err != nil {
		return "", err
	}

	qqIndex := map[string]bool{}
	for _, f := range qqFiles {
		qqIndex[filesFileKey(f)] = true
	}
	folderIndex := map[string]string{}
	for _, fd := range qqFolders {
		folderIndex[fd.FolderName] = fd.FolderID
	}

	// create missing QQ folders
	var created []string
	for key := range local {
		dir := filepath.Dir(key)
		if dir == "." {
			dir = ""
		}
		if dir != "" && folderIndex[dir] == "" {
			if err := api.CreateGroupFileFolder(groupID, dir, "/"); err != nil {
				log.Printf("[files] create folder %s: %v", dir, err)
				continue
			}
			folderIndex[dir] = dir
			created = append(created, dir)
		}
	}
	// refresh folder IDs after creation
	if len(created) > 0 {
		root, _ := api.GetGroupRootFiles(groupID)
		if root != nil {
			for _, fd := range root.Folders {
				folderIndex[fd.FolderName] = fd.FolderID
			}
		}
	}

	var jobs []fileJob
	for key, path := range local {
		key, path := key, path
		if qqIndex[key] {
			continue
		}
		jobs = append(jobs, func() (string, error) {
			dir := filepath.Dir(key)
			if dir == "." {
				dir = ""
			}
			folderID := folderIndex[dir]
			return key, api.UploadGroupFile(groupID, path, filepath.Base(key), folderID)
		})
	}

	ok, fail := runConcurrent(jobs, filesMaxUpload)
	msg := fmt.Sprintf("上传 %d 个，失败 %d 个", len(ok), len(fail))
	if len(created) > 0 {
		msg += fmt.Sprintf("\n新建文件夹：%s", strings.Join(created, "、"))
	}
	return msg, nil
}

// ── Service: clear ───────────────────────────────────────────────────────────

func filesClear(api *bot.BotAPI, groupID int64, exts []string) (string, error) {
	extSet := map[string]bool{}
	for _, e := range exts {
		extSet[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}

	files, _, err := filesGetAll(api, groupID, true)
	if err != nil {
		return "", err
	}

	var ok, fail int
	for _, f := range files {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.FileName), "."))
		if !extSet[ext] {
			continue
		}
		if err := api.DeleteGroupFile(groupID, f.FileID, f.BusID); err != nil {
			log.Printf("[files] delete %s: %v", f.FileName, err)
			fail++
		} else {
			ok++
		}
	}
	return fmt.Sprintf("删除 %d 个，失败 %d 个（扩展名：%s）", ok, fail, strings.Join(exts, "、")), nil
}

// ── Service: organize ────────────────────────────────────────────────────────

func filesOrganize(api *bot.BotAPI, groupID int64, targetFolder string, exts []string) (string, error) {
	extSet := map[string]bool{}
	for _, e := range exts {
		extSet[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}

	root, err := api.GetGroupRootFiles(groupID)
	if err != nil {
		return "", err
	}

	// find or create target folder
	var targetID string
	for _, fd := range root.Folders {
		if fd.FolderName == targetFolder {
			targetID = fd.FolderID
			break
		}
	}
	created := false
	if targetID == "" {
		if err := api.CreateGroupFileFolder(groupID, targetFolder, "/"); err != nil {
			return "", fmt.Errorf("创建文件夹失败：%v", err)
		}
		created = true
		// refresh
		root, _ = api.GetGroupRootFiles(groupID)
		for _, fd := range root.Folders {
			if fd.FolderName == targetFolder {
				targetID = fd.FolderID
				break
			}
		}
	}

	var ok, fail int
	for _, f := range root.Files {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.FileName), "."))
		if !extSet[ext] {
			continue
		}
		if err := api.MoveGroupFile(groupID, f.FileID, "/", targetID); err != nil {
			log.Printf("[files] move %s: %v", f.FileName, err)
			fail++
		} else {
			ok++
		}
	}

	msg := fmt.Sprintf("移动 %d 个，失败 %d 个 → %s", ok, fail, targetFolder)
	if created {
		msg += "（新建文件夹）"
	}
	return msg, nil
}

// ── Service: query ───────────────────────────────────────────────────────────

func filesQuery(api *bot.BotAPI, groupID int64, keyword string) (string, error) {
	files, _, err := filesGetAll(api, groupID, true)
	if err != nil {
		return "", err
	}

	key := filesNormalize(keyword)
	var results []string
	for _, f := range files {
		if strings.Contains(filesNormalize(f.FileName), key) {
			folder := f.FolderName
			if folder == "" {
				folder = "根目录"
			}
			sizeMB := float64(f.FileSize) / 1024 / 1024
			results = append(results, fmt.Sprintf("%s\n  文件夹: %s  大小: %.2fMB", f.FileName, folder, sizeMB))
			if len(results) >= 5 {
				break
			}
		}
	}
	if len(results) == 0 {
		return "未找到匹配的文件", nil
	}
	return "查询结果（最多5个）:\n" + strings.Join(results, "\n"), nil
}

// ── Util ─────────────────────────────────────────────────────────────────────

func defaultKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
