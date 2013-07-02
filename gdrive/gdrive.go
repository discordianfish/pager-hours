package gdrive

import (
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/drive/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	clientId          = "1096729988422-khtfqkcpmti8o5ebjaj0qo5u5rfbej6o.apps.googleusercontent.com"
	redirectUrl       = "urn:ietf:wg:oauth:2.0:oob"
	directoryMimeType = "application/vnd.google-apps.folder"
)

var (
	ErrMultiple = errors.New("More than one file found")
)

type gdrive struct {
	transport *oauth.Transport
}

func New(secret, refreshToken, code string) (*gdrive, error) {
	if secret == "" {
		return nil, fmt.Errorf("Need secret to continue")
	}
	scopes := []string{drive.DriveFileScope, drive.DriveScope}
	config := &oauth.Config{
		ClientId:     clientId,
		ClientSecret: secret,
		RedirectURL:  redirectUrl,
		Scope:        strings.Join(scopes, " "),
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}

	d := &gdrive{
		transport: &oauth.Transport{
			Config: config,
		},
	}

	if refreshToken != "" {
		d.transport.Token = &oauth.Token{RefreshToken: refreshToken}
		d.transport.Refresh()
		log.Printf("New token! Access: '%s', Refresh: '%s', Expires: %s",
			d.transport.Token.AccessToken, d.transport.Token.RefreshToken, d.transport.Token.Expiry)
		return d, nil
	}

	if code == "" {
		return nil, fmt.Errorf("You need to either provide refreshToken or code. For a new code, visit: %s", config.AuthCodeURL(""))
	}

	token, err := d.transport.Exchange(code)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get token: %s", err)
	}
	log.Printf("New token! Access: '%s', Refresh: '%s', Expires: %s",
		token.AccessToken, token.RefreshToken, token.Expiry)

	d.transport.Token = token

	return d, nil
}

func (d *gdrive) NewToken(code string) (*oauth.Token, error) {
	return d.transport.Exchange(code)
}

func (d *gdrive) Drive() (*drive.Service, error) {
	return drive.New(d.transport.Client())
}

func (d *gdrive) Upload(file io.Reader, parentId string, title string) (*drive.File, error) {
	pref := &drive.ParentReference{Id: parentId}
	gd, err := d.Drive()
	if err != nil {
		return nil, err
	}

	metadata := &drive.File{Title: title, Parents: []*drive.ParentReference{pref}}
	return gd.Files.Insert(metadata).Convert(true).Media(file).Do()
}

func (d *gdrive) Find(query string) (*drive.FileList, error) {
	gd, err := d.Drive()
	if err != nil {
		return nil, err
	}
	return gd.Files.List().Q(query).Do()
}

func (d *gdrive) CreateDirectory(name, parent string) (*drive.File, error) {
	gd, err := d.Drive()
	if err != nil {
		return nil, err
	}
	metadata := &drive.File{
		Title:    name,
		Parents:  []*drive.ParentReference{{Id: parent}},
		MimeType: directoryMimeType,
	}
	return gd.Files.Insert(metadata).Do()
}

func (d *gdrive) GetOrCreateDirectory(name, parent string) (*drive.File, error) {
	files, err := d.Find(fmt.Sprintf("title = '%s' and '%s' in parents and trashed = false", name, parent))
	if err != nil {
		return nil, err
	}

	n := len(files.Items)
	if n > 1 {
		return nil, ErrMultiple
	}

	if n == 0 {
		return d.CreateDirectory(name, parent)
	}

	return files.Items[0], nil
}
