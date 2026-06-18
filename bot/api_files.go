package bot

import "encoding/json"

type QQFile struct {
	GroupID    int64  `json:"group_id"`
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	BusID      int    `json:"busid"`
	FileSize   int64  `json:"file_size"`
	FolderName string `json:"-"` // populated by caller
}

type QQFolder struct {
	GroupID    int64  `json:"group_id"`
	FolderID   string `json:"folder_id"`
	FolderName string `json:"folder_name"`
}

type GroupFileSnapshot struct {
	Files   []QQFile
	Folders []QQFolder
}

type groupFileList struct {
	Files   []QQFile   `json:"files"`
	Folders []QQFolder `json:"folders"`
}

func (a *BotAPI) GetGroupRootFiles(groupID int64) (*GroupFileSnapshot, error) {
	raw, err := a.call("get_group_root_files", map[string]any{
		"group_id":   groupID,
		"file_count": 9999,
	})
	if err != nil {
		return nil, err
	}
	var list groupFileList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return &GroupFileSnapshot{Files: list.Files, Folders: list.Folders}, nil
}

func (a *BotAPI) GetGroupFilesByFolder(groupID int64, folderID string) (*GroupFileSnapshot, error) {
	raw, err := a.call("get_group_files_by_folder", map[string]any{
		"group_id":   groupID,
		"folder_id":  folderID,
		"file_count": 9999,
	})
	if err != nil {
		return nil, err
	}
	var list groupFileList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return &GroupFileSnapshot{Files: list.Files, Folders: list.Folders}, nil
}

func (a *BotAPI) GetGroupFileURL(groupID int64, fileID string, busid int) (string, error) {
	raw, err := a.call("get_group_file_url", map[string]any{
		"group_id": groupID,
		"file_id":  fileID,
		"busid":    busid,
	})
	if err != nil {
		return "", err
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return result.URL, nil
}

func (a *BotAPI) CreateGroupFileFolder(groupID int64, name, parentID string) error {
	if parentID == "" {
		parentID = "/"
	}
	_, err := a.call("create_group_file_folder", map[string]any{
		"group_id":  groupID,
		"name":      name,
		"parent_id": parentID,
	})
	return err
}

func (a *BotAPI) DeleteGroupFile(groupID int64, fileID string, busid int) error {
	_, err := a.call("delete_group_file", map[string]any{
		"group_id": groupID,
		"file_id":  fileID,
		"busid":    busid,
	})
	return err
}

func (a *BotAPI) MoveGroupFile(groupID int64, fileID, currentParent, targetParent string) error {
	_, err := a.call("move_group_file", map[string]any{
		"group_id":                 groupID,
		"file_id":                  fileID,
		"current_parent_directory": currentParent,
		"target_parent_directory":  targetParent,
	})
	return err
}

func (a *BotAPI) UploadGroupFile(groupID int64, filePath, name string, folderID string) error {
	params := map[string]any{
		"group_id": groupID,
		"file":     filePath,
		"name":     name,
	}
	if folderID != "" {
		params["folder"] = folderID
	}
	_, err := a.callT("upload_group_file", params, uploadCallTimeout)
	return err
}
