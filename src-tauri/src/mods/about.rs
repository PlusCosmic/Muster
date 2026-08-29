//! Parsing of RimWorld `About/About.xml` mod metadata.
//!
//! Tag matching is case-insensitive; every package id we emit is lowercased and
//! de-duplicated while preserving document order.

use roxmltree::{Document, Node};

/// Everything we care about from an `About.xml`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct AboutData {
    pub package_id: String,
    pub name: String,
    pub authors: String,
    pub supported_versions: Vec<String>,
    pub dependencies: Vec<String>,
    pub load_after: Vec<String>,
    pub load_before: Vec<String>,
    pub force_load_after: Vec<String>,
    pub force_load_before: Vec<String>,
    pub incompatible_with: Vec<String>,
}

/// Strip a UTF-8 BOM and any leading whitespace so roxmltree sees the
/// declaration first.
fn clean(xml: &str) -> &str {
    xml.trim_start_matches('\u{feff}').trim_start()
}

fn tag_eq(node: &Node, name: &str) -> bool {
    node.is_element() && node.tag_name().name().eq_ignore_ascii_case(name)
}

fn child<'a, 'input: 'a>(parent: Node<'a, 'input>, name: &str) -> Option<Node<'a, 'input>> {
    parent.children().find(|c| tag_eq(c, name))
}

fn children<'a, 'input: 'a>(parent: Node<'a, 'input>, name: &str) -> Vec<Node<'a, 'input>> {
    parent.children().filter(|c| tag_eq(c, name)).collect()
}

fn text_of(node: Node) -> String {
    // `text()` only sees the first text child; concatenate for safety.
    let mut s = String::new();
    for d in node.descendants() {
        if d.is_text() {
            s.push_str(d.text().unwrap_or(""));
        }
    }
    s.trim().to_string()
}

fn push_id(out: &mut Vec<String>, raw: &str) {
    let id = raw.trim().to_ascii_lowercase();
    if id.is_empty() || out.iter().any(|e| e == &id) {
        return;
    }
    out.push(id);
}

/// Collect `<li>` text values from `<parent><li>id</li>…</parent>`.
fn collect_li_ids(parent: Node, out: &mut Vec<String>) {
    for li in children(parent, "li") {
        push_id(out, &text_of(li));
    }
}

/// Collect `<li><packageId>id</packageId>…</li>` entries (modDependencies shape).
fn collect_dep_ids(parent: Node, out: &mut Vec<String>) {
    for li in children(parent, "li") {
        if let Some(pid) = child(li, "packageId") {
            push_id(out, &text_of(pid));
        } else {
            // Some mods write a bare `<li>id</li>` even under modDependencies.
            let t = text_of(li);
            if !t.contains('\n') {
                push_id(out, &t);
            }
        }
    }
}

/// `<xByVersion><v1.6>…</v1.6></xByVersion>` — merge every version block, since
/// which one applies depends on the running game and being over-inclusive only
/// adds ordering constraints, never removes them.
fn collect_by_version<F: Fn(Node, &mut Vec<String>)>(parent: Node, out: &mut Vec<String>, f: F) {
    for v in parent.children().filter(|c| c.is_element()) {
        f(v, out);
    }
}

/// Parse an About.xml document. `Err` means the file is malformed or has no
/// usable identity; callers skip the mod and log.
pub fn parse_about(xml: &str) -> Result<AboutData, String> {
    let doc = Document::parse(clean(xml)).map_err(|e| format!("xml parse error: {e}"))?;
    let root = doc.root_element();

    let mut about = AboutData::default();

    for node in root.children().filter(|c| c.is_element()) {
        let tag = node.tag_name().name().to_ascii_lowercase();
        match tag.as_str() {
            "packageid" => about.package_id = text_of(node).to_ascii_lowercase(),
            "name" => about.name = text_of(node),
            "author" => {
                if about.authors.is_empty() {
                    about.authors = text_of(node)
                }
            }
            "authors" => {
                let list: Vec<String> = children(node, "li")
                    .into_iter()
                    .map(text_of)
                    .filter(|s| !s.is_empty())
                    .collect();
                if !list.is_empty() {
                    about.authors = list.join(", ");
                }
            }
            "supportedversions" => {
                for li in children(node, "li") {
                    let v = text_of(li);
                    if !v.is_empty() && !about.supported_versions.contains(&v) {
                        about.supported_versions.push(v);
                    }
                }
            }
            "moddependencies" => collect_dep_ids(node, &mut about.dependencies),
            "moddependenciesbyversion" => {
                collect_by_version(node, &mut about.dependencies, collect_dep_ids)
            }
            "loadafter" => collect_li_ids(node, &mut about.load_after),
            "loadafterbyversion" => collect_by_version(node, &mut about.load_after, collect_li_ids),
            "loadbefore" => collect_li_ids(node, &mut about.load_before),
            "loadbeforebyversion" => {
                collect_by_version(node, &mut about.load_before, collect_li_ids)
            }
            "forceloadafter" => collect_li_ids(node, &mut about.force_load_after),
            "forceloadbefore" => collect_li_ids(node, &mut about.force_load_before),
            "incompatiblewith" => collect_li_ids(node, &mut about.incompatible_with),
            "incompatiblewithbyversion" => {
                collect_by_version(node, &mut about.incompatible_with, collect_li_ids)
            }
            _ => {}
        }
    }

    if about.package_id.is_empty() {
        return Err("missing <packageId>".into());
    }
    if about.name.is_empty() {
        about.name = about.package_id.clone();
    }

    Ok(about)
}

#[cfg(test)]
mod tests {
    use super::*;

    const FULL: &str = r#"<?xml version="1.0" encoding="utf-8"?>
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
</ModMetaData>"#;

    #[test]
    fn parses_all_fields_lowercasing_ids() {
        let a = parse_about(FULL).expect("should parse");
        assert_eq!(a.package_id, "author.fancymod");
        assert_eq!(a.name, "Fancy Mod & Friends");
        assert_eq!(a.authors, "Alice, Bob");
        assert_eq!(a.supported_versions, vec!["1.5", "1.6"]);
        assert_eq!(a.dependencies, vec!["brrainz.harmony"]);
        // duplicate differing only by case collapses to one entry
        assert_eq!(a.load_after, vec!["ludeon.rimworld"]);
        assert_eq!(a.load_before, vec!["some.other"]);
        assert_eq!(a.force_load_after, vec!["force.after"]);
        assert_eq!(a.force_load_before, vec!["force.before"]);
        assert_eq!(a.incompatible_with, vec!["bad.mod"]);
    }

    #[test]
    fn accepts_bom_single_author_and_case_variant_tags() {
        let xml = "\u{feff}<?xml version=\"1.0\"?>\n<modmetadata><PACKAGEID>A.B</PACKAGEID>\
                   <Name>N</Name><Author>Solo Dev</Author></modmetadata>";
        let a = parse_about(xml).unwrap();
        assert_eq!(a.package_id, "a.b");
        assert_eq!(a.name, "N");
        assert_eq!(a.authors, "Solo Dev");
        assert!(a.supported_versions.is_empty());
    }

    #[test]
    fn merges_by_version_blocks() {
        let xml = r#"<ModMetaData>
          <packageId>x.y</packageId><name>XY</name>
          <modDependenciesByVersion>
            <v1.5><li><packageId>Dep.One</packageId></li></v1.5>
            <v1.6><li><packageId>Dep.Two</packageId></li></v1.6>
          </modDependenciesByVersion>
          <loadAfterByVersion><v1.6><li>After.One</li></v1.6></loadAfterByVersion>
        </ModMetaData>"#;
        let a = parse_about(xml).unwrap();
        assert_eq!(a.dependencies, vec!["dep.one", "dep.two"]);
        assert_eq!(a.load_after, vec!["after.one"]);
    }

    #[test]
    fn rejects_malformed_and_identityless_xml() {
        assert!(parse_about("<ModMetaData><name>oops</ModMetaData>").is_err());
        assert!(parse_about("<ModMetaData><name>no id</name></ModMetaData>").is_err());
    }

    #[test]
    fn name_falls_back_to_package_id() {
        let a = parse_about("<ModMetaData><packageId>Only.Id</packageId></ModMetaData>").unwrap();
        assert_eq!(a.name, "only.id");
    }
}
