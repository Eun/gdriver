package gdriver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/Eun/gdriver/oauthhelper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func setup(t *testing.T) (*GDriver, func()) {
	helper := oauthhelper.Auth{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Authenticate: func(url string) (string, error) {
			return "", fmt.Errorf("please specify a valid token.json file")
		},
	}
	var err error
	var client *http.Client
	var driver *GDriver
	var token []byte

	token, err = base64.StdEncoding.DecodeString(os.Getenv("GOOGLE_TOKEN"))
	require.NoError(t, err)

	helper.Token = new(oauth2.Token)

	require.NoError(t, json.Unmarshal([]byte(token), helper.Token))

	client, err = helper.NewHTTPClient(context.Background())
	require.NoError(t, err)

	driver, err = New(client)

	require.NoError(t, err)

	// prepare test directory

	fullPath := sanitizeName(fmt.Sprintf("GDriveTest-%s", t.Name()))
	driver.DeleteDirectory(fullPath)
	_, err = driver.MakeDirectory(fullPath)
	require.NoError(t, err)

	_, err = driver.SetRootDirectory(fullPath)
	require.NoError(t, err)

	return driver, func() {
		_, err = driver.SetRootDirectory("")
		require.NoError(t, err)
		require.NoError(t, driver.DeleteDirectory(fullPath))
	}
}

func TestMakeDirectory(t *testing.T) {
	t.Run("simple creation", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1")))

		// Folder1 created?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("simple creation in existent directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1")))

		// Folder1 created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		require.NoError(t, getError(driver.MakeDirectory("Folder1/Folder2")))

		// Folder1/Folder2 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2")))
	})

	t.Run("create non existent directories", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1/Folder2/Folder3")))

		// Folder1 created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// Folder1/Folder2 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2")))

		// Folder1/Folder2/Folder3 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2/Folder3")))
	})

	t.Run("create folder as a descendant of a file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		require.EqualError(t, getError(driver.MakeDirectory("Folder1/File1/Folder2")), "unable to create directory in `Folder1/File1': `File1' is not a directory")
	})

	t.Run("make root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("")))
	})
}

func TestPutFile(t *testing.T) {
	t.Run("in root folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create file
		require.NoError(t, getError(driver.PutFile("File1", bytes.NewBufferString("Hello World"))))

		// file created?
		require.NoError(t, getError(driver.Stat("File1")))

		// Compare file contents
		_, r, err := driver.GetFile("File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.Equal(t, "Hello World", string(received))
	})

	t.Run("in non existing folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create file
		require.NoError(t, getError(driver.PutFile("Folder1/File1", bytes.NewBufferString("Hello World"))))

		// Folder created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// File created?
		require.NoError(t, getError(driver.Stat("Folder1/File1")))

		// Compare file contents
		_, r, err := driver.GetFile("Folder1/File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.Equal(t, "Hello World", string(received))
	})

	t.Run("as descendant of file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create file
		require.NoError(t, getError(driver.PutFile("Folder1/File1", bytes.NewBufferString("Hello World"))))

		require.EqualError(t, getError(driver.PutFile("Folder1/File1/File2", bytes.NewBufferString("Hello World"))), "unable to create file in `Folder1/File1': `File1' is not a directory")
	})

	t.Run("empty target", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create file
		require.EqualError(t, getError(driver.PutFile("", bytes.NewBufferString("Hello World"))), "path cannot be empty")
	})
}

func TestGetFile(t *testing.T) {
	driver, teardown := setup(t)
	defer teardown()

	newFile(t, driver, "Folder1/File1", "Hello World")

	// Compare file contents
	_, r, err := driver.GetFile("Folder1/File1")
	received, err := ioutil.ReadAll(r)
	require.Equal(t, "Hello World", string(received))

	// Get file contents of an Folder
	_, _, err = driver.GetFile("Folder1")
	require.EqualError(t, err, "`Folder1' is a directory")
}

func TestDelete(t *testing.T) {
	t.Run("Delete File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		// delete file
		require.NoError(t, driver.Delete("File1"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("File1")), "`File1' not found")
	})

	t.Run("Delete Directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.Delete("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' not found")
	})
}

func TestDeleteDirectory(t *testing.T) {
	t.Run("Delete File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		// delete file
		require.EqualError(t, driver.DeleteDirectory("File1"), "`File1' is not a directory")

		// file  should not be deleted
		require.NoError(t, getError(driver.Stat("File1")))
	})

	t.Run("Delete Directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.DeleteDirectory("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' not found")
	})
}

func TestListDirectory(t *testing.T) {
	driver, teardown := setup(t)
	defer teardown()

	newFile(t, driver, "Folder1/File1", "Hello World")
	newFile(t, driver, "Folder1/File2", "Hello World")

	var files []*FileInfo
	require.NoError(t, driver.ListDirectory("Folder1", func(f *FileInfo) error {
		files = append(files, f)
		return nil
	}))

	require.Len(t, files, 2)

	require.True(t, (files[0].Name() == "File1" && files[1].Name() == "File2") || (files[0].Name() == "File2" && files[1].Name() == "File1"))

	// Delete contents
	require.NoError(t, driver.Delete("Folder1/File1"))
	require.NoError(t, driver.Delete("Folder1/File2"))

	// File1 deleted?
	require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' not found")

	// File2 deleted?
	require.EqualError(t, getError(driver.Stat("Folder1/File2")), "`Folder1/File2' not found")

	// Test if folder is empty
	files = []*FileInfo{}
	require.NoError(t, driver.ListDirectory("Folder1", func(f *FileInfo) error {
		files = append(files, f)
		return nil
	}))

	require.Len(t, files, 0)

	// Delete folder
	require.NoError(t, driver.DeleteDirectory("Folder1"))

	// Folder deleted?
	require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' not found")

	files = []*FileInfo{}
	require.EqualError(t, driver.ListDirectory("Folder1", func(f *FileInfo) error {
		files = append(files, f)
		return nil
	}), "`Folder1' not found")
}

func TestRename(t *testing.T) {
	t.Run("rename with simple name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// rename
		require.NoError(t, getError(driver.Rename("Folder1/File1", "File2")))

		// file renamed?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' not found")
	})

	t.Run("rename with path", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// rename
		require.NoError(t, getError(driver.Rename("Folder1/File1", "Folder2/File2")))

		// file renamed?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' not found")

		// Folder2 should not have been created
		require.EqualError(t, getError(driver.Stat("Folder2")), "`Folder2' not found")
	})

	t.Run("rename directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1")))

		// rename
		require.NoError(t, getError(driver.Rename("Folder1", "Folder2")))

		// Folder2 renamed?
		require.NoError(t, getError(driver.Stat("Folder2")))

		// old folder gone?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' not found")
	})

	t.Run("invalid new name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		require.EqualError(t, getError(driver.Rename("Folder1/File1", "")), "new name cannot be empty")
	})

	t.Run("rename root node", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Rename("/", "Test")), "root cannot be renamed")
	})
}

func TestMove(t *testing.T) {
	t.Run("Move Into Another Folder with another Name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move file
		require.NoError(t, getError(driver.Move("Folder1/File1", "Folder2/File2")))

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File2")))

		// Old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' not found")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("Move Into Another Folder with same Name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move file
		require.NoError(t, getError(driver.Move("Folder1/File1", "Folder2/File1")))

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File1")))

		// Old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' not found")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("Move Into Same Folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move file
		require.NoError(t, getError(driver.Move("Folder1/File1", "Folder1/File2")))

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// Old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' not found")
	})

	t.Run("Move Root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Move("", "Folder1")), "root cannot be moved")
	})

	t.Run("Invalid target", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Move("Folder1", "")), "new path cannot be empty")
	})
}

func newFile(t *testing.T, driver *GDriver, path, contents string) {
	_, err := driver.PutFile(path, bytes.NewBufferString(contents))
	require.NoError(t, err)
}

func newDirectory(t *testing.T, driver *GDriver, path string) {
	_, err := driver.MakeDirectory(path)
	require.NoError(t, err)
}

func getError(_ *FileInfo, err error) error {
	return err
}
