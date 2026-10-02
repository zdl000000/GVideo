import { Upload } from "lucide-react";
import { Link } from "react-router-dom";

export function StudioEmptyState() {
  return <div className="gv-studio-empty"><span className="eyebrow">CONTENT / 00</span><h2>还没有投稿</h2><p>发布第一条作品，内容状态和表现会显示在这里。</p><Link className="primary-button" to="/upload"><Upload size={17} />发布第一条作品</Link></div>;
}
