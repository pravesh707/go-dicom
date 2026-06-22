// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/pravesh707/go-dicom/dicom"
)

func TestTagString(t *testing.T) {
	cases := map[dicom.Tag]string{
		dicom.NewTag(0x0000, 0x0000): "(0000,0000)",
		dicom.NewTag(0x7FE0, 0x0010): "(7FE0,0010)",
		dicom.NewTag(0x0010, 0x0010): "(0010,0010)",
		dicom.NewTag(0xFFFE, 0xE0DD): "(FFFE,E0DD)",
	}
	for tag, want := range cases {
		if got := tag.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", tag, got, want)
		}
	}
}

func TestTagClassification(t *testing.T) {
	if !(dicom.NewTag(0x0000, 0x0100)).IsCommand() {
		t.Error("command tag not recognized")
	}
	if (dicom.NewTag(0x0008, 0x0018)).IsCommand() {
		t.Error("data tag wrongly recognized as command")
	}
	if !(dicom.NewTag(0x0008, 0x0000)).IsGroupLength() {
		t.Error("group length tag not recognized")
	}
	if (dicom.NewTag(0x0008, 0x0018)).IsGroupLength() {
		t.Error("non-group-length tag wrongly recognized")
	}
	// Private = odd group, excluding the special 0001/0003.
	if !(dicom.NewTag(0x0009, 0x0010)).IsPrivate() {
		t.Error("odd group should be private")
	}
	if (dicom.NewTag(0x0008, 0x0010)).IsPrivate() {
		t.Error("even group should not be private")
	}
	if (dicom.NewTag(0x0001, 0x0010)).IsPrivate() {
		t.Error("group 0001 excluded from private")
	}
}

func TestTagLess(t *testing.T) {
	if !(dicom.NewTag(0x0008, 0x0018)).Less(dicom.NewTag(0x0008, 0x0020)) {
		t.Error("element ordering within group failed")
	}
	if (dicom.NewTag(0x0008, 0x0020)).Less(dicom.NewTag(0x0008, 0x0018)) {
		t.Error("reverse element ordering failed")
	}
	if !(dicom.NewTag(0x0007, 0xFFFF)).Less(dicom.NewTag(0x0008, 0x0000)) {
		t.Error("group ordering failed")
	}
	if (dicom.NewTag(0x0008, 0x0000)).Less(dicom.NewTag(0x0008, 0x0000)) {
		t.Error("equal tags should not be Less")
	}
}

func TestVRLongLength(t *testing.T) {
	long := []dicom.VR{dicom.VROB, dicom.VROD, dicom.VROF, dicom.VROL, dicom.VROV, dicom.VROW, dicom.VRSQ, dicom.VRUC, dicom.VRUN, dicom.VRUR, dicom.VRUT, dicom.VRSV, dicom.VRUV}
	for _, vr := range long {
		if !vr.UsesLongLength() {
			t.Errorf("VR %s should use long length", vr)
		}
	}
	short := []dicom.VR{dicom.VRAE, dicom.VRUS, dicom.VRUL, dicom.VRPN, dicom.VRUI, dicom.VRCS, dicom.VRDA, dicom.VRFL, dicom.VRFD, dicom.VRSS, dicom.VRSL}
	for _, vr := range short {
		if vr.UsesLongLength() {
			t.Errorf("VR %s should use short length", vr)
		}
	}
}

func TestVRPadByte(t *testing.T) {
	if dicom.VRUI.PadByte() != 0x00 {
		t.Error("UI pads with NUL")
	}
	if dicom.VROB.PadByte() != 0x00 {
		t.Error("OB pads with NUL")
	}
	for _, vr := range []dicom.VR{dicom.VRPN, dicom.VRCS, dicom.VRLO, dicom.VRSH, dicom.VRDA, dicom.VRTM, dicom.VRUT} {
		if vr.PadByte() != 0x20 {
			t.Errorf("%s should pad with space", vr)
		}
	}
}

func TestVRIsString(t *testing.T) {
	if !dicom.VRPN.IsString() || !dicom.VRUI.IsString() || !dicom.VRCS.IsString() {
		t.Error("text VRs should be strings")
	}
	if dicom.VROB.IsString() || dicom.VRUS.IsString() || dicom.VRFL.IsString() {
		t.Error("binary VRs should not be strings")
	}
}

func TestTransferSyntaxName(t *testing.T) {
	if dicom.TransferSyntaxName(dicom.ImplicitVRLittleEndian) != "Implicit VR Little Endian" {
		t.Error("implicit name wrong")
	}
	if dicom.TransferSyntaxName(dicom.ExplicitVRLittleEndian) != "Explicit VR Little Endian" {
		t.Error("explicit name wrong")
	}
	if got := dicom.TransferSyntaxName("1.2.3.unknown"); got != "1.2.3.unknown" {
		t.Errorf("unknown TS should echo UID, got %q", got)
	}
}

func TestIsImplicitVR(t *testing.T) {
	if !dicom.IsImplicitVR(dicom.ImplicitVRLittleEndian) {
		t.Error("implicit not detected")
	}
	if dicom.IsImplicitVR(dicom.ExplicitVRLittleEndian) {
		t.Error("explicit wrongly detected as implicit")
	}
}

func TestLookupVR(t *testing.T) {
	if dicom.LookupVR(dicom.TagCommandField) != dicom.VRUS {
		t.Error("command field VR")
	}
	if dicom.LookupVR(dicom.NewTag(0x0008, 0x0000)) != dicom.VRUL {
		t.Error("unknown group-length should be UL")
	}
	if dicom.LookupVR(dicom.NewTag(0x0009, 0x0010)) != dicom.VRUN {
		t.Error("unknown tag should be UN")
	}
}

func TestRegisterDictEntry(t *testing.T) {
	tag := dicom.NewTag(0x4321, 0x1234)
	if _, ok := dicom.LookupEntry(tag); ok {
		t.Skip("tag unexpectedly present")
	}
	dicom.RegisterDictEntry(tag, dicom.DictEntry{VR: dicom.VRLO, VM: "1", Keyword: "TestPrivate"})
	if dicom.LookupVR(tag) != dicom.VRLO {
		t.Error("registered VR not returned")
	}
	e, ok := dicom.LookupEntry(tag)
	if !ok || e.Keyword != "TestPrivate" {
		t.Errorf("LookupEntry = %+v, %v", e, ok)
	}
}

func TestElementNumericAccessors(t *testing.T) {
	us := dicom.NewUS(dicom.NewTag(0x0028, 0x0010), 1, 2, 3)
	if us.Len() != 6 {
		t.Errorf("US len = %d, want 6", us.Len())
	}
	if us.Uint16() != 1 {
		t.Errorf("US first = %d", us.Uint16())
	}
	got := us.Uint16s()
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("US values = %v", got)
	}

	ul := dicom.NewUL(dicom.TagCommandGroupLength, 0x01020304)
	if ul.Uint32() != 0x01020304 {
		t.Errorf("UL = %#x", ul.Uint32())
	}

	// Short raw is handled gracefully.
	short := &dicom.Element{Tag: dicom.NewTag(1, 1), VR: dicom.VRUS, Raw: []byte{0x01}}
	if short.Uint16() != 0 {
		t.Error("short US should return 0")
	}
	if (&dicom.Element{VR: dicom.VRUL, Raw: []byte{1, 2}}).Uint32() != 0 {
		t.Error("short UL should return 0")
	}
}

func TestElementStrings(t *testing.T) {
	ui := dicom.NewUI(dicom.NewTag(0x0008, 0x0018), "1.2.3") // odd -> padded with NUL
	if ui.Len()%2 != 0 || ui.Raw[ui.Len()-1] != 0x00 {
		t.Error("UI not NUL-padded to even")
	}
	if ui.String() != "1.2.3" {
		t.Errorf("UI String = %q", ui.String())
	}

	multi := dicom.NewString(dicom.NewTag(0x0008, 0x0060), dicom.VRCS, "A", "B")
	if multi.Len()%2 != 0 {
		t.Error("CS not padded even")
	}
	if multi.String() != `A\B` {
		t.Errorf("CS String = %q", multi.String())
	}
	vals := multi.Strings()
	if len(vals) != 2 || vals[0] != "A" || vals[1] != "B" {
		t.Errorf("CS Strings = %v", vals)
	}

	if got := dicom.NewString(dicom.NewTag(0x0010, 0x0010), dicom.VRPN, "").Strings(); got != nil {
		t.Errorf("empty Strings should be nil, got %v", got)
	}
}

func TestDataSetOps(t *testing.T) {
	ds := dicom.NewDataSet()
	if ds.Len() != 0 {
		t.Error("new dataset not empty")
	}
	ds.Set(dicom.NewString(dicom.NewTag(0x0010, 0x0010), dicom.VRPN, "Doe"))
	ds.Set(dicom.NewUI(dicom.NewTag(0x0008, 0x0018), "1.2"))
	ds.Set(dicom.NewUI(dicom.NewTag(0x0008, 0x0016), "1.1"))

	if ds.Len() != 3 {
		t.Errorf("len = %d", ds.Len())
	}
	if !ds.Has(dicom.NewTag(0x0010, 0x0010)) {
		t.Error("Has missing")
	}

	// Tags must be ascending by (group, element).
	tags := ds.Tags()
	want := []dicom.Tag{dicom.NewTag(0x0008, 0x0016), dicom.NewTag(0x0008, 0x0018), dicom.NewTag(0x0010, 0x0010)}
	for i := range want {
		if tags[i] != want[i] {
			t.Errorf("tag[%d] = %v, want %v", i, tags[i], want[i])
		}
	}
	if els := ds.Elements(); len(els) != 3 || els[0].Tag != want[0] {
		t.Error("Elements order mismatch")
	}

	if name, ok := ds.GetString(dicom.NewTag(0x0010, 0x0010)); !ok || name != "Doe" {
		t.Errorf("GetString = %q, %v", name, ok)
	}
	if _, ok := ds.GetUint16(dicom.NewTag(0x9999, 0x9999)); ok {
		t.Error("absent GetUint16 should be false")
	}

	ds.Remove(dicom.NewTag(0x0010, 0x0010))
	if ds.Has(dicom.NewTag(0x0010, 0x0010)) {
		t.Error("Remove failed")
	}
}
