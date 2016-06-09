package sqlstore

import (
	"fmt"
	"testing"
	"time"

	m "github.com/raintank/worldping-api/pkg/models"
	. "github.com/smartystreets/goconvey/convey"
)

func TestProbes(t *testing.T) {
	InitTestDB(t)
	probeCount := 0
	Convey("When adding probe", t, func() {
		pre := time.Now()
		p := &m.ProbeDTO{
			Name:      fmt.Sprintf("test%d", probeCount),
			OrgId:     1,
			Tags:      []string{"test", "dev"},
			Public:    false,
			Latitude:  1.0,
			Longitude: 1.0,
			Online:    false,
			Enabled:   true,
		}
		err := AddProbe(p)
		So(err, ShouldBeNil)
		probeCount++
		So(p.Id, ShouldNotEqual, 0)
		So(p.Name, ShouldEqual, fmt.Sprintf("test%d", probeCount-1))
		So(p.OrgId, ShouldEqual, 1)
		So(len(p.Tags), ShouldEqual, 2)
		So(p.Tags, ShouldContain, "test")
		So(p.Tags, ShouldContain, "dev")
		So(p.Public, ShouldEqual, false)
		So(p.Online, ShouldEqual, false)
		So(p.Created.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
		So(p.Created.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
		So(p.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
		So(p.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
		So(p.OnlineChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
		So(p.OnlineChange.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
		Convey("When replacing Probe tags", func() {
			p.Tags = []string{"foo", "bar"}
			err := UpdateProbe(p)
			So(err, ShouldBeNil)
			So(p.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, p.Created.Unix())
			So(p.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			Convey("Tags should be updated in DB", func() {
				updated, err := GetProbeById(p.Id, p.OrgId)
				So(err, ShouldBeNil)
				So(len(updated.Tags), ShouldEqual, 2)
				So(updated.Tags, ShouldContain, "foo")
				So(updated.Tags, ShouldContain, "bar")
			})

		})
		Convey("When removing probe tag", func() {
			p.Tags = []string{"test"}
			err := UpdateProbe(p)
			So(err, ShouldBeNil)
			So(p.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, p.Created.Unix())
			So(p.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			Convey("Tags should be updated in DB", func() {
				updated, err := GetProbeById(p.Id, p.OrgId)
				So(err, ShouldBeNil)
				So(len(updated.Tags), ShouldEqual, 1)
				So(updated.Tags, ShouldContain, "test")
			})
		})
		Convey("When adding additional probe tags", func() {
			p.Tags = []string{"test", "dev", "foo"}
			err := UpdateProbe(p)
			So(err, ShouldBeNil)
			So(p.Updated.Unix(), ShouldBeGreaterThanOrEqualTo, p.Created.Unix())
			So(p.Updated.Unix(), ShouldBeLessThanOrEqualTo, time.Now().Unix())
			Convey("Tags should be updated in DB", func() {
				updated, err := GetProbeById(p.Id, p.OrgId)
				So(err, ShouldBeNil)
				So(len(updated.Tags), ShouldEqual, 3)
				So(updated.Tags, ShouldContain, "foo")
				So(updated.Tags, ShouldContain, "dev")
				So(updated.Tags, ShouldContain, "test")
			})
		})
		Convey("When deleting probe", func() {
			err := DeleteProbe(p.Id, p.OrgId)
			So(err, ShouldBeNil)
			probeCount--
			Convey("When listing probes for org with probes", func() {
				probes, err := GetProbes(&m.GetProbesQuery{
					OrgId: 1,
				})
				So(err, ShouldBeNil)
				So(len(probes), ShouldEqual, probeCount)
			})
		})
		Convey("When listing probes for org with no probes", func() {
			probes, err := GetProbes(&m.GetProbesQuery{
				OrgId: 123,
			})
			So(err, ShouldBeNil)
			So(len(probes), ShouldEqual, 0)
		})
	})
}

func TestProbeSessions(t *testing.T) {
	InitTestDB(t)
	p := &m.ProbeDTO{
		Name:      fmt.Sprintf("test%d", 1),
		OrgId:     1,
		Tags:      []string{"test", "dev"},
		Public:    false,
		Latitude:  1.0,
		Longitude: 1.0,
		Online:    false,
		Enabled:   true,
	}
	err := AddProbe(p)
	if err != nil {
		t.Fatal(err)
	}
	Convey("When adding probeSession", t, func() {
		pre := time.Now()
		session := m.ProbeSession{
			OrgId:      1,
			ProbeId:    p.Id,
			SocketId:   "sid1",
			Version:    "1.0.0",
			InstanceId: "default",
			RemoteIp:   "127.0.0.1",
		}
		err := AddProbeSession(&session)
		So(err, ShouldBeNil)
		So(session.Id, ShouldNotEqual, 0)

		Convey("new session should set probe to online", func() {
			probe, err := GetProbeById(p.Id, p.OrgId)
			So(err, ShouldBeNil)
			So(probe, ShouldNotBeNil)
			So(probe.Online, ShouldEqual, true)
			So(probe.OnlineChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())

			Convey("when deleting probeSesson", func() {
				err := DeleteProbeSession(&session)
				So(err, ShouldBeNil)
				Convey("probe should be offline again", func() {
					probe, err := GetProbeById(p.Id, p.OrgId)
					So(err, ShouldBeNil)
					So(probe, ShouldNotBeNil)
					So(probe.Online, ShouldEqual, false)
					So(probe.OnlineChange.Unix(), ShouldBeGreaterThanOrEqualTo, pre.Unix())
				})
			})
		})
	})
}
