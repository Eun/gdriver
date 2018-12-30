# GDriver
A golang implementation to access google drive by using traditional file-folder-path pattern.

```go
    f, _ := os.Open("image1.jpeg")
    gdrive.PutFile("Holidays/image1.jpeg", f)
```

```go
    gdrive.Delete("Pictures/Old Apartment Images")
```
[Example](example/main.go)