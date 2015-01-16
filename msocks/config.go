package msocks

import (
	"errors"
	"math/rand"
	"time"

	"github.com/op/go-logging"
)

const (
	PINGTIME       = 10000
	PINGRANDOM     = 2000
	TIMEOUT_COUNT  = 6
	GAMEOVER_COUNT = 60

	DIAL_RETRY   = 3
	DIAL_TIMEOUT = 30000

	WINDOWSIZE = 4 * 1024 * 1024

	AUTH_TIMEOUT      = 10000
	MIN_SESS_NUM      = 2
	MAX_CONN_PRE_SESS = 8

	SHRINK_TIME = 5000
	SHRINK_RATE = 0.9
)

const (
	ERR_NONE = iota
	ERR_AUTH
	ERR_IDEXIST
	ERR_CONNFAILED
	ERR_TIMEOUT
	ERR_CLOSED
)

var (
	ErrNoSession       = errors.New("session in pool but can't pick one.")
	ErrStreamNotExist  = errors.New("stream not exist.")
	ErrQueueClosed     = errors.New("queue closed")
	ErrUnexpectedPkg   = errors.New("unexpected package")
	ErrNotSyn          = errors.New("frame result in conn which status is not syn")
	ErrFinState        = errors.New("status not est or fin wait when get fin")
	ErrIdExist         = errors.New("frame sync stream id exist.")
	ErrState           = errors.New("status error")
	ErrSessionNotFound = errors.New("session not found")
)

var (
	log        = logging.MustGetLogger("msocks")
	frame_ping = NewFramePing()
)

func init() {
	rand.Seed(time.Now().UnixNano())
}
