package gdriver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// GDriver can be used to access google drive in a traditional file-folder-path pattern
type GDriver struct {
	srv      *drive.Service
	rootNode *FileInfo
}

// HashMethod is the hashing method to use for GetFileHash
type HashMethod int

const (
	// HashMethodMD5 sets the method to MD5
	HashMethodMD5 HashMethod = 0
)

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
	mimeTypeFile   = "application/octet-stream"
)

var (
	fileInfoFields []googleapi.Field
	listFields     []googleapi.Field
)

func init() {
	fileInfoFields = []googleapi.Field{
		"createdTime",
		"id",
		"mimeType",
		"modifiedTime",
		"name",
		"size",
	}
	listFields = []googleapi.Field{
		googleapi.Field(fmt.Sprintf("files(%s)", googleapi.CombineFields(fileInfoFields))),
	}
}

// New creates a new Google Drive Driver, client must me an authenticated instance for google drive
func New(client *http.Client) (*GDriver, error) {
	srv, err := drive.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Drive client: %v", err)
	}

	driver := &GDriver{
		srv: srv,
	}

	if _, err = driver.SetRootDirectory(""); err != nil {
		return nil, err
	}

	return driver, nil
}

// SetRootDirectory changes the working root directory
// use this if you want to do certian operations in a special directory
// path should always be the absolute real path
func (d *GDriver) SetRootDirectory(path string) (*FileInfo, error) {
	rootNode, err := getRootNode(d.srv)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Drive root: %v", err)
	}

	file, err := d.getFile(rootNode, path, listFields...)
	if err != nil {
		return nil, err
	}
	if !file.IsDir() {
		return nil, fmt.Errorf("`%s' is not a directory", path)
	}
	d.rootNode = file
	return file, nil
}

// Stat gives a FileInfo for a file or directory
func (d *GDriver) Stat(path string) (*FileInfo, error) {
	return d.getFile(d.rootNode, path, listFields...)
}

// ListDirectory will get all contents of a directory, calling fileFunc with the collected file information
func (d *GDriver) ListDirectory(path string, fileFunc func(*FileInfo) error) error {
	file, err := d.getFile(d.rootNode, path, "files(id,name,mimeType)")
	if err != nil {
		return err
	}
	if !file.IsDir() {
		return fmt.Errorf("`%s' is not a directory", path)
	}
	descendants, err := d.srv.Files.List().Q(fmt.Sprintf("'%s' in parents and trashed = false", file.item.Id)).Fields(listFields...).Do()
	if err != nil {
		return err
	}

	if descendants == nil {
		return fmt.Errorf("no file information present (in `%s')", path)
	}

	for i := 0; i < len(descendants.Files); i++ {
		if err = fileFunc(&FileInfo{
			item:       descendants.Files[i],
			parentPath: file.Path(),
		}); err != nil {
			return CallbackError{NestedError: err}
		}
	}
	return nil
}

// MakeDirectory creates a directory for the specified path, it will create non existent directores automatically
//
// Examples:
//     MakeDirectory("Pictures/Holidays") // will create Pictures and Holidays
func (d *GDriver) MakeDirectory(path string) (*FileInfo, error) {
	return d.makeDirectoryByParts(strings.FieldsFunc(path, isPathSeperator))
}

func (d *GDriver) makeDirectoryByParts(pathParts []string) (*FileInfo, error) {
	parentNode := d.rootNode
	for i := 0; i < len(pathParts); i++ {
		query := fmt.Sprintf("'%s' in parents and name='%s' and trashed = false", parentNode.item.Id, sanitizeName(pathParts[i]))
		files, err := d.srv.Files.List().Q(query).Fields(listFields...).Do()
		if err != nil {
			return nil, err
		}
		if files == nil {
			return nil, fmt.Errorf("no file information present (in `%s')", path.Join(pathParts[:i+1]...))
		}

		if len(files.Files) <= 0 {
			// file not found => create directory
			if !parentNode.IsDir() {
				return nil, fmt.Errorf("unable to create directory in `%s': `%s' is not a directory", path.Join(pathParts[:i]...), parentNode.Name())
			}
			var createdDir *drive.File
			createdDir, err = d.srv.Files.Create(&drive.File{
				Name:     sanitizeName(pathParts[i]),
				MimeType: mimeTypeFolder,
				Parents: []string{
					parentNode.item.Id,
				},
			}).Fields(fileInfoFields...).Do()
			if err != nil {
				return nil, err
			}
			parentNode = &FileInfo{
				item:       createdDir,
				parentPath: path.Join(pathParts[:i]...),
			}
		} else if len(files.Files) > 1 {
			return nil, fmt.Errorf("multiple entries found for `%s'", path.Join(pathParts[:i+1]...))
		} else { // if len(files.Files) == 1
			parentNode = &FileInfo{
				item:       files.Files[0],
				parentPath: path.Join(pathParts[:i]...),
			}
		}
	}
	return parentNode, nil
}

// DeleteDirectory will delete a directory and its descendants
func (d *GDriver) DeleteDirectory(path string) error {
	file, err := d.getFile(d.rootNode, path, "files(id,mimeType)")
	if err != nil {
		return err
	}
	if !file.IsDir() {
		return fmt.Errorf("`%s' is not a directory", path)
	}

	if file == d.rootNode {
		return errors.New("root cannot be deleted")
	}
	return d.srv.Files.Delete(file.item.Id).Do()
}

// Delete will delete a file or directory, if directory it will also delete its descendants
func (d *GDriver) Delete(path string) error {
	file, err := d.getFile(d.rootNode, path)
	if err != nil {
		return err
	}
	if file == d.rootNode {
		return errors.New("root cannot be deleted")
	}
	return d.srv.Files.Delete(file.item.Id).Do()
}

// GetFile gets a file and returns a ReadCloser that can consume the body of the file
func (d *GDriver) GetFile(path string) (*FileInfo, io.ReadCloser, error) {
	file, err := d.getFile(d.rootNode, path, listFields...)
	if err != nil {
		return nil, nil, err
	}
	if file.IsDir() {
		return nil, nil, fmt.Errorf("`%s' is a directory", path)
	}

	response, err := d.srv.Files.Get(file.item.Id).Download()
	if err != nil {
		return nil, nil, err
	}

	return file, response.Body, nil
}

// GetFileHash returns the hash of a file with the present method
func (d *GDriver) GetFileHash(path string, method HashMethod) (*FileInfo, []byte, error) {
	switch method {
	case HashMethodMD5:
	default:
		return nil, nil, fmt.Errorf("Unknown method %d", method)
	}
	file, err := d.getFile(d.rootNode, path, "files(id, md5Checksum)")
	if err != nil {
		return nil, nil, err
	}
	if file.IsDir() {
		return nil, nil, fmt.Errorf("`%s' is a directory", path)
	}

	return file, []byte(file.item.Md5Checksum), nil
}

// PutFile uploads a file to the specified path
// it creates non existing directories
func (d *GDriver) PutFile(filePath string, r io.Reader) (*FileInfo, error) {
	pathParts := strings.FieldsFunc(filePath, isPathSeperator)
	amountOfParts := len(pathParts)
	if amountOfParts <= 0 {
		return nil, errors.New("path cannot be empty")
	}

	parentNode := d.rootNode
	if amountOfParts > 1 {
		dir, err := d.makeDirectoryByParts(pathParts[:amountOfParts-1])
		if err != nil {
			return nil, err
		}
		parentNode = dir

		if !parentNode.IsDir() {
			return nil, fmt.Errorf("unable to create file in `%s': `%s' is not a directory", path.Join(pathParts[:amountOfParts-1]...), parentNode.Name())
		}
	}

	file, err := d.srv.Files.Create(
		&drive.File{
			Name:     sanitizeName(pathParts[amountOfParts-1]),
			MimeType: mimeTypeFile,
			Parents: []string{
				parentNode.item.Id,
			},
		},
	).Fields(fileInfoFields...).Media(r).Do()
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		item:       file,
		parentPath: path.Join(pathParts[:amountOfParts-1]...),
	}, nil
}

// Rename renames a file or directory to a new name in the same folder
func (d *GDriver) Rename(path string, newName string) (*FileInfo, error) {
	newNameParts := strings.FieldsFunc(newName, isPathSeperator)
	amountOfParts := len(newNameParts)
	if amountOfParts <= 0 {
		return nil, errors.New("new name cannot be empty")
	}
	file, err := d.getFile(d.rootNode, path)
	if err != nil {
		return nil, err
	}

	if file == d.rootNode {
		return nil, errors.New("root cannot be renamed")
	}

	newFile, err := d.srv.Files.Update(file.item.Id, &drive.File{
		Name: sanitizeName(newNameParts[amountOfParts-1]),
	}).Fields(fileInfoFields...).Do()
	return &FileInfo{
		item:       newFile,
		parentPath: file.parentPath,
	}, nil
}

// Move moves a file or directory to a new path, note that move also renames the target if necessary and creates non existing directories
//
// Examples:
//     Move("Folder1/File1", "Folder2/File2") // File1 in Folder1 will be moved to Folder2/File2
//     Move("Folder1/File1", "Folder2/File1") // File1 in Folder1 will be moved to Folder2/File1
func (d *GDriver) Move(oldPath, newPath string) (*FileInfo, error) {
	pathParts := strings.FieldsFunc(newPath, isPathSeperator)
	amountOfParts := len(pathParts)
	if amountOfParts <= 0 {
		return nil, errors.New("new path cannot be empty")
	}

	file, err := d.getFile(d.rootNode, oldPath, "files(id,parents)")
	if err != nil {
		return nil, err
	}

	if file == d.rootNode {
		return nil, errors.New("root cannot be moved")
	}

	parentNode := d.rootNode
	if amountOfParts > 1 {
		dir, err := d.makeDirectoryByParts(pathParts[:amountOfParts-1])
		if err != nil {
			return nil, err
		}
		parentNode = dir

		if !parentNode.IsDir() {
			return nil, fmt.Errorf("unable to create file in `%s': `%s' is not a directory", path.Join(pathParts[:amountOfParts-1]...), parentNode.Name())
		}
	}

	newFile, err := d.srv.Files.Update(file.item.Id, &drive.File{
		Name: sanitizeName(pathParts[amountOfParts-1]),
	}).
		AddParents(parentNode.item.Id).
		RemoveParents(path.Join(file.item.Parents...)).
		Fields(fileInfoFields...).Do()
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		item:       newFile,
		parentPath: path.Join(pathParts[:amountOfParts-1]...),
	}, nil
}

// Trash trashes a file or directory
func (d *GDriver) Trash(path string) error {
	file, err := d.getFile(d.rootNode, path, "files(id)")
	if err != nil {
		return err
	}

	if file == d.rootNode {
		return errors.New("root cannot be trashed")
	}

	_, err = d.srv.Files.Update(file.item.Id, &drive.File{
		Trashed: true,
	}).Do()
	return err
}

// ListTrash lists the contents of the trash, if you specify directories it will only list the trash contents of the specified directories
func (d *GDriver) ListTrash(filePath string, fileFunc func(f *FileInfo) error) error {
	file, err := d.getFile(d.rootNode, filePath, "files(id,name)")
	if err != nil {
		return err
	}

	// no directories specified
	files, err := d.srv.Files.List().Q("trashed = true").Fields(googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields)))).Do()
	if err != nil {
		return err
	}

	for i := 0; i < len(files.Files); i++ {
		// determinate the parent of this file

		inRoot, parentPath, err := isInRoot(d.srv, file.item.Id, files.Files[i], "")
		if err != nil {
			return err
		}

		if inRoot {
			if err = fileFunc(&FileInfo{
				item:       files.Files[i],
				parentPath: path.Join(file.Path(), parentPath),
			}); err != nil {
				return CallbackError{NestedError: err}
			}
		}
	}
	return nil
}

func getRootNode(srv *drive.Service) (*FileInfo, error) {
	root, err := srv.Files.Get("root").Fields(fileInfoFields...).Do()
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		item:       root,
		parentPath: "",
	}, nil
}

// isInRoot checks if a file is a descendant of root, if so it will return the parent path of the file
func isInRoot(srv *drive.Service, rootID string, file *drive.File, basePath string) (bool, string, error) {
	for _, parentID := range file.Parents {
		if parentID == rootID {
			return true, basePath, nil
		}
		parent, err := srv.Files.Get(parentID).Fields("id,name,parents").Do()
		if err != nil {
			return false, "", err
		}
		if inRoot, parentPath, err := isInRoot(srv, rootID, parent, path.Join(parent.Name, basePath)); err != nil || inRoot {
			return inRoot, parentPath, err
		}
	}
	return false, "", nil
}

func (d *GDriver) getFile(rootNode *FileInfo, path string, fields ...googleapi.Field) (*FileInfo, error) {
	return d.getFileByParts(rootNode, strings.FieldsFunc(path, isPathSeperator), fields...)
}

func (d *GDriver) getFileByParts(rootNode *FileInfo, pathParts []string, fields ...googleapi.Field) (*FileInfo, error) {
	amountOfParts := len(pathParts)

	if amountOfParts == 0 {
		// get root directory if we have no parts
		return rootNode, nil
	}

	lastID := rootNode.item.Id
	lastPart := amountOfParts - 1
	var lastFile *drive.File
	for i := 0; i < amountOfParts; i++ {
		query := fmt.Sprintf("'%s' in parents and name='%s' and trashed = false", lastID, sanitizeName(pathParts[i]))
		// log.Println(query)
		call := d.srv.Files.List().Q(query)

		// if we are not at the last part
		if i == lastPart {
			if len(fields) <= 0 {
				call = call.Fields("files(id)")
			} else {
				call = call.Fields(fields...)
			}
		} else {
			call = call.Fields("files(id)")
		}
		files, err := call.Do()
		if err != nil {
			return nil, err
		}
		if files == nil || len(files.Files) <= 0 {
			return nil, NotFoundError{Path: path.Join(pathParts[:i+1]...)}
		}
		if len(files.Files) > 1 {
			return nil, fmt.Errorf("multiple entries found for `%s'", path.Join(pathParts[:i+1]...))
		}
		lastFile = files.Files[0]
		lastID = lastFile.Id
		// log.Printf("=>%s = %s\n", path.Join(pathParts[:i+1]...), lastID)
	}

	return &FileInfo{
		item:       lastFile,
		parentPath: path.Join(pathParts[:amountOfParts-1]...),
	}, nil
}
