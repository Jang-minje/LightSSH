import type { ConnectionProfile } from "../api/profiles";

type ConnectionProfileCardProps = {
  profile: ConnectionProfile;
  onEdit: () => void;
  onDelete: () => void;
};

export function ConnectionProfileCard({
  profile,
  onEdit,
  onDelete,
}: ConnectionProfileCardProps) {
  return (
    <article className="profile-card">
      <div className="profile-main">
        <h2>{profile.name}</h2>
        <p>
          {profile.username}@{profile.host}:{profile.port}
        </p>
      </div>
      <div className="profile-actions">
        <button
          type="button"
          className="profile-edit"
          onClick={onEdit}
          title="프로필 수정"
        >
          선택
        </button>
        <button
          type="button"
          className="profile-delete"
          onClick={onDelete}
          title="프로필 삭제"
        >
          삭제
        </button>
      </div>
    </article>
  );
}
