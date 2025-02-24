package fileswap

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gtank/cryptopasta"
	"github.com/joomcode/errorx"
	"github.com/minio/sio"

	"github.com/pingcap/tidb-dashboard/util/nocopy"
	"github.com/pingcap/tidb-dashboard/util/rest/resterror"
)

// Handler provides a file-based data serving HTTP handler.
// Arbitrary data stream can be stored in the file in encrypted form temporarily, and then downloaded by the user later.
// As data is stored in the file, large chunk of data is supported.
//
// Note: the download token cannot be mixed in different Handler instances.
type Handler struct {
	nocopy.NoCopy

	// The secret is used to sign the download token as well as encrypting the file in the FS.
	secret []byte
}

func New() *Handler {
	return &Handler{
		secret: cryptopasta.NewEncryptionKey()[:],
	}
}

// NewFileWriter creates a writer for storing data into FS. A download token can be generated from the writer
// for downloading later. The downloading can be handled by the HandleDownloadRequest.
func (s *Handler) NewFileWriter(tempFilePattern string) (*FileWriter, error) {
	file, err := ioutil.TempFile("", tempFilePattern)
	if err != nil {
		return nil, err
	}

	w, err := sio.EncryptWriter(file, sio.Config{Key: s.secret})
	if err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}

	return &FileWriter{
		WriteCloser: w,
		secret:      s.secret,
		filePath:    file.Name(),
	}, nil
}

type downloadTokenClaims struct {
	jwt.StandardClaims
	TempFileName     string
	DownloadFileName string
}

func (s *Handler) parseClaimsFromToken(tokenString string) (*downloadTokenClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&downloadTokenClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return s.secret, nil
		})
	if token != nil {
		if claims, ok := token.Claims.(*downloadTokenClaims); ok && token.Valid {
			return claims, nil
		}
	}
	if ve, ok := err.(*jwt.ValidationError); ok && ve.Errors&jwt.ValidationErrorExpired != 0 {
		return nil, errorx.Decorate(err, "download token is expired")
	}
	return nil, errorx.Decorate(err, "download token is invalid")
}

// HandleDownloadRequest handles a gin Request for serving the file in the FS by using a download token.
// The file will be removed after it is successfully served to the user.
func (s *Handler) HandleDownloadRequest(c *gin.Context) {
	claims, err := s.parseClaimsFromToken(c.Query("token"))
	if err != nil {
		_ = c.Error(resterror.ErrBadRequest.Wrap(err, "Invalid download request"))
		return
	}

	file, err := os.Open(claims.TempFileName)
	if err != nil {
		if os.IsNotExist(err) {
			// It is possible that token is reused. In this case, raise invalid request error.
			_ = c.Error(resterror.ErrBadRequest.Wrap(err, "Download file not found. Please retry."))
		} else {
			_ = c.Error(err)
		}
		return
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(claims.TempFileName)
	}()

	c.Writer.Header().Set("Content-type", "application/octet-stream")
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, claims.DownloadFileName))

	_, err = sio.Decrypt(c.Writer, file, sio.Config{
		Key: s.secret,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
}

type FileWriter struct {
	nocopy.NoCopy
	io.WriteCloser

	secret   []byte
	filePath string
}

func (fw *FileWriter) Remove() {
	_ = fw.Close()
	_ = os.Remove(fw.filePath)
}

// GetDownloadToken generates a download token for downloading this file later.
// The downloading can be handled by the Handler.HandleDownloadRequest.
func (fw *FileWriter) GetDownloadToken(downloadFileName string, expireIn time.Duration) (string, error) {
	claims := downloadTokenClaims{
		TempFileName:     fw.filePath,
		DownloadFileName: downloadFileName,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(expireIn).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenSigned, err := token.SignedString(fw.secret)
	if err != nil {
		return "", err
	}
	return tokenSigned, nil
}
