package capture

import (
	"image"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

func cursor() (image.Point, bool) {
	conn, err := xgb.NewConn()
	if err != nil {
		return image.Point{}, false
	}
	defer conn.Close()
	root := xproto.Setup(conn).DefaultScreen(conn).Root
	reply, err := xproto.QueryPointer(conn, root).Reply()
	if err != nil {
		return image.Point{}, false
	}
	return image.Pt(int(reply.RootX), int(reply.RootY)), true
}
