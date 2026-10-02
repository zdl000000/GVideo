import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import { errorMessage } from "../../shared/lib/errors";
import { pageFrom } from "../../shared/lib/format";
import type { VideoPage } from "../../types";

export function useDiscoveryFeed(sort: "latest" | "popular") {
  const [params, setParams] = useSearchParams();
  const query = params.get("q") || "";
  const category = params.get("category") || "";
  const page = pageFrom(params);
  const requestKey = JSON.stringify([sort, query, category, page]);
  const [loadedKey, setLoadedKey] = useState("");
  const [result, setResult] = useState<VideoPage>({ items: [], page: 1, page_size: 36, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [categoryError, setCategoryError] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let active = true;
    api.categories().then((items) => { if (active) { setCategories(items); setCategoryError(""); } }).catch(() => { if (active) setCategoryError("分类暂时无法加载，你仍可以浏览或搜索视频。"); });
    return () => { active = false; };
  }, [revision]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    const request = new URLSearchParams({ sort, page: String(page), page_size: "36" });
    if (query) request.set("q", query);
    if (category) request.set("category", category);
    api.videos(request, controller.signal).then((data) => { if (!controller.signal.aborted) { setResult(data); setLoadedKey(requestKey); } }).catch((err) => { if (!controller.signal.aborted) { setError(errorMessage(err)); setLoadedKey(requestKey); } }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [sort, query, category, page, revision, requestKey]);

  const setCategory = (value: string) => {
    const next = new URLSearchParams(params);
    value ? next.set("category", value) : next.delete("category");
    next.delete("sort");
    next.delete("page");
    setParams(next);
  };
  const setPage = (value: number) => {
    const next = new URLSearchParams(params);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  return { params, query, category, page, result, categories, categoryError, loading: loading || loadedKey !== requestKey, error, setCategory, setPage, retry: () => setRevision((value) => value + 1) };
}
