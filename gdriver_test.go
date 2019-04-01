package gdriver

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Eun/gdriver/oauthhelper"
	"github.com/hjson/hjson-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

func setup(t *testing.T) (*GDriver, func()) {
	env, err := ioutil.ReadFile(".env.json")
	if err != nil {
		if !os.IsNotExist(err) {
			require.NoError(t, err)
		}
	}
	if len(env) > 0 {
		var environmentVariables map[string]interface{}
		require.NoError(t, hjson.Unmarshal(env, &environmentVariables))
		for key, val := range environmentVariables {
			if s, ok := val.(string); ok {
				require.NoError(t, os.Setenv(key, s))
			} else {
				require.FailNow(t, "unable to set environment", "Key `%s' is not a string was a %T", key, val)
			}
		}
	}

	helper := oauthhelper.Auth{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Authenticate: func(url string) (string, error) {
			return "", fmt.Errorf("please specify a valid token.json file")
		},
	}
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

		fi, err := driver.MakeDirectory("Folder1")
		require.NoError(t, err)
		require.Equal(t, "Folder1", fi.Path())

		// Folder1 created?
		fi, err = driver.Stat("Folder1")
		require.NoError(t, err)
		require.Equal(t, "Folder1", fi.Path())
	})

	t.Run("simple creation in existent directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1")))

		fi, err := driver.MakeDirectory("Folder1/Folder2")
		require.NoError(t, err)
		require.Equal(t, "Folder1/Folder2", fi.Path())

		// Folder1/Folder2 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2")))
	})

	t.Run("create non existent directories", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		fi, err := driver.MakeDirectory("Folder1/Folder2/Folder3")
		require.NoError(t, err)
		require.Equal(t, "Folder1/Folder2/Folder3", fi.Path())

		// Folder1 created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// Folder1/Folder2 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2")))

		// Folder1/Folder2/Folder3 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2/Folder3")))
	})

	t.Run("creation of existent directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1/Folder2")))

		fi, err := driver.MakeDirectory("Folder1/Folder2")
		require.NoError(t, err)
		require.Equal(t, "Folder1/Folder2", fi.Path())
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
		fi, err := driver.PutFile("File1", bytes.NewBufferString("Hello World"))
		require.NoError(t, err)
		require.Equal(t, "File1", fi.Path())

		// file created?
		fi, err = driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi.Path())

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
		fi, err := driver.PutFile("Folder1/File1", bytes.NewBufferString("Hello World"))
		require.NoError(t, err)
		require.Equal(t, "Folder1/File1", fi.Path())

		// Folder created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// File created?
		fi, err = driver.Stat("Folder1/File1")
		require.NoError(t, err)
		require.Equal(t, "Folder1/File1", fi.Path())

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

	t.Run("overwrite file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create file
		fi1, err := driver.PutFile("File1", bytes.NewBufferString("Hello World"))
		require.NoError(t, err)
		require.Equal(t, "File1", fi1.Path())

		// file created?
		fi1, err = driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi1.Path())

		// Compare file contents
		_, r, err := driver.GetFile("File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.Equal(t, "Hello World", string(received))

		// create file
		fi2, err := driver.PutFile("File1", bytes.NewBufferString("Hello Universe"))
		require.NoError(t, err)
		require.Equal(t, "File1", fi2.Path())

		// file created?
		fi2, err = driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi2.Path())

		// Compare file contents
		_, r, err = driver.GetFile("File1")
		require.NoError(t, err)
		received, err = ioutil.ReadAll(r)
		require.Equal(t, "Hello Universe", string(received))
	})
}

func TestGetFile(t *testing.T) {
	driver, teardown := setup(t)
	defer teardown()

	newFile(t, driver, "Folder1/File1", "Hello World")

	// Compare file contents
	fi, r, err := driver.GetFile("Folder1/File1")
	require.NoError(t, err)
	received, err := ioutil.ReadAll(r)
	require.Equal(t, "Hello World", string(received))
	require.Equal(t, "Folder1/File1", fi.Path())

	// Get file contents of an Folder
	_, _, err = driver.GetFile("Folder1")
	require.EqualError(t, err, "`Folder1' is a directory")
}

func TestDelete(t *testing.T) {
	t.Run("delete file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		// delete file
		require.NoError(t, driver.Delete("File1"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("File1")), "`File1' does not exist")
	})

	t.Run("delete directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.Delete("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})
}

func TestDeleteDirectory(t *testing.T) {
	t.Run("delete file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		// delete file
		require.EqualError(t, driver.DeleteDirectory("File1"), "`File1' is not a directory")

		// file  should not be deleted
		require.NoError(t, getError(driver.Stat("File1")))
	})

	t.Run("delete directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.DeleteDirectory("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})
}

func TestListDirectory(t *testing.T) {
	t.Run("standart", func(t *testing.T) {
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

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, "Folder1/File1", files[0].Path())
		require.Equal(t, "Folder1/File2", files[1].Path())

		// Delete contents
		require.NoError(t, driver.Delete("Folder1/File1"))
		require.NoError(t, driver.Delete("Folder1/File2"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// File2 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1/File2")), "`Folder1/File2' does not exist")

		// Test if folder is empty
		files = []*FileInfo{}
		require.NoError(t, driver.ListDirectory("Folder1", func(f *FileInfo) error {
			files = append(files, f)
			return nil
		}))

		require.Len(t, files, 0)
	})

	t.Run("directory does not exist", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, driver.ListDirectory("Folder1", func(f *FileInfo) error {
			return nil
		}), "`Folder1' does not exist")
	})

	t.Run("list file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		require.EqualError(t, driver.ListDirectory("File1", func(f *FileInfo) error {
			return nil
		}), "`File1' is not a directory")
	})

	t.Run("callback error", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		err := driver.ListDirectory("", func(f *FileInfo) error {
			return errors.New("Custom Error")
		})
		require.IsType(t, CallbackError{}, err)
		require.EqualError(t, err, "callback throwed an error: Custom Error")
	})
}

func TestRename(t *testing.T) {
	t.Run("rename with simple name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// rename
		fi, err := driver.Rename("Folder1/File1", "File2")
		require.NoError(t, err)
		require.Equal(t, "Folder1/File2", fi.Path())

		// file renamed?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")
	})

	t.Run("rename with path", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// rename
		fi, err := driver.Rename("Folder1/File1", "Folder2/File2")
		require.NoError(t, err)
		require.Equal(t, "Folder1/File2", fi.Path())

		// file renamed?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Folder2 should not have been created
		require.EqualError(t, getError(driver.Stat("Folder2")), "`Folder2' does not exist")
	})

	t.Run("rename directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, getError(driver.MakeDirectory("Folder1")))

		// rename
		fi, err := driver.Rename("Folder1", "Folder2")
		require.NoError(t, err)
		require.Equal(t, "Folder2", fi.Path())

		// Folder2 renamed?
		require.NoError(t, getError(driver.Stat("Folder2")))

		// old folder gone?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
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
	t.Run("move into another folder with another name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move file
		fi, err := driver.Move("Folder1/File1", "Folder2/File2")
		require.NoError(t, err)
		require.Equal(t, "Folder2/File2", fi.Path())

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File2")))

		// Old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("move into another folder with same name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move file
		fi, err := driver.Move("Folder1/File1", "Folder2/File1")
		require.NoError(t, err)
		require.Equal(t, "Folder2/File1", fi.Path())

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File1")))

		// Old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("move into same folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move file
		fi, err := driver.Move("Folder1/File1", "Folder1/File2")
		require.NoError(t, err)
		require.Equal(t, "Folder1/File2", fi.Path())

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// Old file gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")
	})

	t.Run("move root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Move("", "Folder1")), "root cannot be moved")
	})

	t.Run("invalid target", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Move("Folder1", "")), "new path cannot be empty")
	})
}

func TestTrash(t *testing.T) {
	t.Run("trash file", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// trash file
		require.NoError(t, driver.Trash("Folder1/File1"))

		// File1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("trash folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// trash folder
		require.NoError(t, driver.Trash("Folder1"))

		// Folder1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")

		// File1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1' does not exist")
	})

	t.Run("trash root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, driver.Trash(""), "root cannot be trashed")
	})
}

func TestListTrash(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		newFile(t, driver, "Folder2/File2", "Hello World")
		newFile(t, driver, "Folder3/File3", "Hello World")

		// trash File1
		require.NoError(t, driver.Trash("Folder1/File1"))
		// trash Folder2
		require.NoError(t, driver.Trash("Folder2"))

		var files []*FileInfo
		require.NoError(t, driver.ListTrash("", func(f *FileInfo) error {
			files = append(files, f)
			return nil
		}))

		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, "Folder1/File1", files[0].Path())
		require.Equal(t, "Folder2", files[1].Path())
	})

	t.Run("of folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		newFile(t, driver, "Folder1/File2", "Hello World")
		newFile(t, driver, "Folder2/File3", "Hello World")

		// trash File1 and File2
		require.NoError(t, driver.Trash("Folder1/File1"))
		require.NoError(t, driver.Trash("Folder1/File2"))

		var files []*FileInfo
		require.NoError(t, driver.ListTrash("Folder1", func(f *FileInfo) error {
			files = append(files, f)
			return nil
		}))

		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, "Folder1/File1", files[0].Path())
		require.Equal(t, "Folder1/File2", files[1].Path())
	})

	t.Run("callback error", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// trash File1
		require.NoError(t, driver.Trash("Folder1/File1"))

		err := driver.ListTrash("", func(f *FileInfo) error {
			return errors.New("Custom Error")
		})
		require.IsType(t, CallbackError{}, err)
		require.EqualError(t, err, "callback throwed an error: Custom Error")
	})
}

func TestIsInRoot(t *testing.T) {
	t.Run("in folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		fi, err := driver.getFile(driver.rootNode, "Folder1/File1", googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields))))
		require.NoError(t, err)

		inRoot, parentPath, err := isInRoot(driver.srv, driver.rootNode.item.Id, fi.item, "")
		require.NoError(t, err)
		require.True(t, inRoot)
		require.Equal(t, "Folder1", parentPath)
	})

	t.Run("not in folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		fi, err := driver.MakeDirectory("Folder2")
		require.NoError(t, err)
		folder2Id := fi.item.Id

		fi, err = driver.getFile(driver.rootNode, "Folder1/File1", googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields))))
		require.NoError(t, err)

		inRoot, parentPath, err := isInRoot(driver.srv, folder2Id, fi.item, "")
		require.NoError(t, err)
		require.False(t, inRoot)
		require.Equal(t, "", parentPath)
	})
}

func TestGetHash(t *testing.T) {
	driver, teardown := setup(t)
	defer teardown()

	buf := bytes.NewBufferString("Hello World")
	hash1 := md5.Sum(buf.Bytes())
	_, err := driver.PutFile("File1", buf)
	require.NoError(t, err)

	_, hash2, err := driver.GetFileHash("File1", HashMethodMD5)
	require.NoError(t, err)

	hash2, err = hex.DecodeString(string(hash2))
	require.NoError(t, err)

	require.EqualValues(t, hash1[:], hash2)
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

func TestOpen(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		t.Run("existing file", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			newFile(t, driver, "Folder1/File1", "Hello World")

			f, err := driver.Open("Folder1/File1", O_RDONLY)
			require.NoError(t, err)
			defer f.Close()

			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, "Hello World", string(data))
		})
		t.Run("non-existing file", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.Open("Folder1/File1", O_RDONLY)
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
		t.Run("non-existing file with create", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.Open("Folder1/File1", O_RDONLY|O_CREATE)
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
	})

	t.Run("write", func(t *testing.T) {
		t.Run("existing file", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			newFile(t, driver, "Folder1/File1", "Hello World")

			f, err := driver.Open("Folder1/File1", O_WRONLY)
			require.NoError(t, err)
			n, err := io.WriteString(f, "Hello Universe")
			require.NoError(t, err)
			require.Equal(t, 14, n)
			require.NoError(t, f.Close())

			// Compare file contents
			_, r, err := driver.GetFile("Folder1/File1")
			require.NoError(t, err)
			received, err := ioutil.ReadAll(r)
			require.Equal(t, "Hello Universe", string(received))
		})
		t.Run("non-existing file", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.Open("Folder1/File1", O_WRONLY)
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
		t.Run("non-existing file with create", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.Open("Folder1/File1", O_WRONLY|O_CREATE)
			n, err := io.WriteString(f, "Hello Universe")
			require.NoError(t, err)
			require.Equal(t, 14, n)
			require.NoError(t, f.Close())

			// Compare file contents
			_, r, err := driver.GetFile("Folder1/File1")
			require.NoError(t, err)
			received, err := ioutil.ReadAll(r)
			require.Equal(t, "Hello Universe", string(received))
		})
	})
}
