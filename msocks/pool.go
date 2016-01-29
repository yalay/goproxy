package msocks

import (
	"net"
	"sync"
)

type AbstractSessionFactory interface {
	CreateSession() (*Session, error)
}

type SessionPool struct {
	mu      sync.Mutex // sess pool & factory locker
	mud     sync.Mutex // dailer's locker
	sess    []*Session
	asfs    []AbstractSessionFactory
	MinSess int
	MaxConn int
}

func CreateSessionPool(MinSess, MaxConn int) (sp *SessionPool) {
	if MinSess == 0 {
		MinSess = 1
	}
	if MaxConn == 0 {
		MaxConn = 16
	}
	sp = &SessionPool{
		sess:    make([]*Session, 0),
		MinSess: MinSess,
		MaxConn: MaxConn,
	}
	return
}

func (sp *SessionPool) AddSessionFactory(sf AbstractSessionFactory) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.asfs = append(sp.asfs, sf)
}

func (sp *SessionPool) CutAll() {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for _, s := range sp.sess {
		s.Close()
	}
	sp.sess = make([]*Session, 0)
}

func (sp *SessionPool) GetSize() int {
	return len(sp.sess)
}

func (sp *SessionPool) GetSessions() (sess []*Session) {
	return sp.sess
}

func (sp *SessionPool) Add(s *Session) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.sess = append(sp.sess, s)
}

func (sp *SessionPool) Remove(s *Session) (n int, err error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i, sess := range sp.sess {
		if s == sess {
			n := len(sp.sess)
			sp.sess[i], sp.sess[n-1] = sp.sess[n-1], sp.sess[i]
			sp.sess = sp.sess[:n-1]
			return len(sp.sess), nil
		}
	}
	return 0, ErrSessionNotFound
}

func (sp *SessionPool) GetSess() (sess *Session, err error) {
	if len(sp.sess) == 0 {
		err = sp.createSession(func() bool {
			return len(sp.sess) == 0
		})
		if err != nil {
			return nil, err
		}
	}

	sess, size := sp.getLessSess()
	if sess == nil {
		return nil, ErrNoSession
	}

	if size > sp.MaxConn || len(sp.sess) < sp.MinSess {
		go sp.createSession(func() bool {
			if len(sp.sess) < sp.MinSess {
				return true
			}
			// normally, size == -1 should never happen
			_, size := sp.getLessSess()
			return size > sp.MaxConn
		})
	}
	return
}

func (sp *SessionPool) createSession(checker func() bool) (err error) {
	sp.mud.Lock()
	defer sp.mud.Unlock()

	if checker != nil && !checker() {
		return
	}

	sess, err := sp.asfs[0].CreateSession()
	if err != nil {
		log.Error("%s", err)
		return
	}

	sp.Add(sess)
	go sp.sessRun(sess)
	return
}

func (sp *SessionPool) getLessSess() (sess *Session, size int) {
	size = -1
	for _, s := range sp.sess {
		if size == -1 || s.GetSize() < size {
			sess = s
			size = s.GetSize()
		}
	}
	return
}

func (sp *SessionPool) sessRun(sess *Session) {
	defer func() {
		_, err := sp.Remove(sess)
		if err != nil {
			log.Error("%s", err)
			return
		}

		// if n < sp.MinSess && !sess.IsGameOver() {
		// 	sp.createSession(func() bool {
		// 		return len(sp.sess) < sp.MinSess
		// 	})
		// }

		// Don't need to check less session here.
		// Mostly, less sess counter in here will not more then the counter in GetOrCreateSess.
		// The only exception is that the closing session is the one and only one
		// lower then max_conn
		// but we can think that as over max_conn line just happened.
	}()

	sess.Run()
	// that's mean session is dead
	log.Warning("session runtime quit, reboot from connect.")
	return
}

func (sp *SessionPool) Dial(network, address string) (net.Conn, error) {
	sess, err := sp.GetSess()
	if err != nil {
		return nil, nil
	}
	return sess.Dial(network, address)
}

func (sp *SessionPool) LookupIP(host string) (addrs []net.IP, err error) {
	sess, err := sp.GetSess()
	if err != nil {
		return
	}
	return sess.LookupIP(host)
}
