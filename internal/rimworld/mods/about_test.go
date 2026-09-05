package mods

import (
	"reflect"
	"testing"
)

const fullAbout = `<?xml version="1.0" encoding="utf-8"?>
<ModMetaData>
  <name>Fancy Mod &amp; Friends</name>
  <authors>
    <li>Alice</li>
    <li>Bob</li>
  </authors>
  <packageId>Author.FancyMod</packageId>
  <supportedVersions>
    <li>1.5</li>
    <li>1.6</li>
  </supportedVersions>
  <modDependencies>
    <li>
      <packageId>Brrainz.Harmony</packageId>
      <displayName>Harmony</displayName>
    </li>
  </modDependencies>
  <loadAfter>
    <li>Ludeon.RimWorld</li>
    <li>ludeon.rimworld</li>
  </loadAfter>
  <loadBefore>
    <li>Some.Other</li>
  </loadBefore>
  <forceLoadAfter>
    <li>Force.After</li>
  </forceLoadAfter>
  <forceLoadBefore>
    <li>Force.Before</li>
  </forceLoadBefore>
  <incompatibleWith>
    <li>Bad.Mod</li>
  </incompatibleWith>
</ModMetaData>`

func eq(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %#v want %#v", what, got, want)
	}
}

func TestParsesAllFieldsLowercasingIDs(t *testing.T) {
	a, err := ParseAbout(fullAbout)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "packageId", a.PackageID, "author.fancymod")
	eq(t, "name", a.Name, "Fancy Mod & Friends")
	eq(t, "authors", a.Authors, "Alice, Bob")
	eq(t, "supportedVersions", a.SupportedVersions, []string{"1.5", "1.6"})
	eq(t, "dependencies", a.Dependencies, []string{"brrainz.harmony"})
	// duplicate differing only by case collapses to one entry
	eq(t, "loadAfter", a.LoadAfter, []string{"ludeon.rimworld"})
	eq(t, "loadBefore", a.LoadBefore, []string{"some.other"})
	eq(t, "forceLoadAfter", a.ForceLoadAfter, []string{"force.after"})
	eq(t, "forceLoadBefore", a.ForceLoadBefore, []string{"force.before"})
	eq(t, "incompatibleWith", a.IncompatibleWith, []string{"bad.mod"})
}

func TestAcceptsBomSingleAuthorAndCaseVariantTags(t *testing.T) {
	body := "\uFEFF<?xml version=\"1.0\"?>\n<modmetadata><PACKAGEID>A.B</PACKAGEID><Name>N</Name><Author>Solo Dev</Author></modmetadata>"
	a, err := ParseAbout(body)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "packageId", a.PackageID, "a.b")
	eq(t, "name", a.Name, "N")
	eq(t, "authors", a.Authors, "Solo Dev")
	if len(a.SupportedVersions) != 0 {
		t.Fatal("expected no supported versions")
	}
}

func TestMergesByVersionBlocks(t *testing.T) {
	body := `<ModMetaData>
          <packageId>x.y</packageId><name>XY</name>
          <modDependenciesByVersion>
            <v1.5><li><packageId>Dep.One</packageId></li></v1.5>
            <v1.6><li><packageId>Dep.Two</packageId></li></v1.6>
          </modDependenciesByVersion>
          <loadAfterByVersion><v1.6><li>After.One</li></v1.6></loadAfterByVersion>
        </ModMetaData>`
	a, err := ParseAbout(body)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "dependencies", a.Dependencies, []string{"dep.one", "dep.two"})
	eq(t, "loadAfter", a.LoadAfter, []string{"after.one"})
}

func TestRejectsMalformedAndIdentitylessXML(t *testing.T) {
	if _, err := ParseAbout("<ModMetaData><name>oops</ModMetaData>"); err == nil {
		t.Fatal("malformed should fail")
	}
	if _, err := ParseAbout("<ModMetaData><name>no id</name></ModMetaData>"); err == nil {
		t.Fatal("missing packageId should fail")
	}
}

func TestNameFallsBackToPackageID(t *testing.T) {
	a, err := ParseAbout("<ModMetaData><packageId>Only.Id</packageId></ModMetaData>")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "name", a.Name, "only.id")
}
