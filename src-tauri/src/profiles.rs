use crate::models::Profile;

pub async fn list_profiles() -> Result<Vec<Profile>, String> {
    Err("not implemented: stream A".into())
}

pub async fn create_profile(_name: String) -> Result<Profile, String> {
    Err("not implemented: stream A".into())
}

pub async fn rename_profile(_id: String, _new_name: String) -> Result<Profile, String> {
    Err("not implemented: stream A".into())
}

pub async fn delete_profile(_id: String) -> Result<(), String> {
    Err("not implemented: stream A".into())
}

pub async fn clone_profile(_id: String, _new_name: String) -> Result<Profile, String> {
    Err("not implemented: stream A".into())
}

pub async fn import_default(_name: String) -> Result<Profile, String> {
    Err("not implemented: stream A".into())
}
