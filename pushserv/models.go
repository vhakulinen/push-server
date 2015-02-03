package pushserv

import (
	"encoding/json"
	"fmt"
	"time"
)

type HttpToken struct {
	Id         int64
	CreatedAt  time.Time
	AccessedAt time.Time

	Token string `sql:"not null;unique"`
	Key   string `sql:"not null;unique"`
}

// Register token for http pooling
func RegisterHttpToken(token, key string) (t *HttpToken, err error) {
	t = new(HttpToken)
	if db.Where("token = ?", token).First(t).RecordNotFound() {
		t = &HttpToken{
			Token:      token,
			Key:        key,
			AccessedAt: time.Now(),
		}
		if err = db.Save(t).Error; err != nil {
			return nil, err
		}
		return t, nil
	}
	return nil, fmt.Errorf("HttpToken already registered")
}

func (t *HttpToken) Delete() {
	db.Delete(t)
}

func (t *HttpToken) GetPushes() []*PushData {
	pushes := []*PushData{}
	db.Where("token = ?", t.Token).Find(&pushes)

	for _, p := range pushes {
		db.Delete(p)
	}
	return pushes
}

type PushData struct {
	Id        int64
	CreatedAt time.Time

	Title string `sql:"not null;unique"`
	Body  string
	Token string `sql:"not null;unique"`
}

func SavePushData(title, body, token string) (p *PushData, err error) {
	p = &PushData{
		Title: title,
		Body:  body,
		Token: token,
	}
	if err = db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PushData) Delete() {
	db.Delete(p)
}

func (p *PushData) ToJson() ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return b, nil
}
