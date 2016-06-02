package sqlstore

import (
	"github.com/go-xorm/xorm"
)

type session struct {
	*xorm.Session
	transaction bool
	complete    bool
}

func newSession(transaction bool, table string) (*session, error) {
	if !transaction {
		return &session{Session: x.Table(table)}, nil
	}
	sess := session{Session: x.NewSession(), transaction: true}
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	sess.Table(table)
	return &sess, nil
}

func (sess *session) Complete() {
	if sess.transaction {
		if err := sess.Commit(); err == nil {
			sess.complete = true
		}
	}
}

func (sess *session) Cleanup() {
	if sess.transaction {
		if !sess.complete {
			sess.Rollback()
		}
		sess.Close()
	}
}
