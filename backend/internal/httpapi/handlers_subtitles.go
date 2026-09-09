package httpapi

import (
	"net/http"
)

func (h *Handler) uploadSubtitle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	if err := r.ParseMultipartForm(3 << 20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "字幕上传内容无效")
		return
	}
	header, err := fileHeader(r, "subtitle", true)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "请选择字幕文件")
		return
	}
	track, err := h.service.AddSubtitle(r.Context(), sessionFrom(r.Context()).User.ID, id, header, r.FormValue("subtitle_language"), r.FormValue("subtitle_label"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, track)
}

func (h *Handler) setDefaultSubtitle(w http.ResponseWriter, r *http.Request) {
	videoID, ok := pathID(w, r)
	if !ok {
		return
	}
	subtitleID, ok := subtitlePathID(w, r)
	if !ok {
		return
	}
	tracks, err := h.service.SetDefaultSubtitle(r.Context(), sessionFrom(r.Context()).User.ID, videoID, subtitleID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, tracks)
}

func (h *Handler) deleteSubtitle(w http.ResponseWriter, r *http.Request) {
	videoID, ok := pathID(w, r)
	if !ok {
		return
	}
	subtitleID, ok := subtitlePathID(w, r)
	if !ok {
		return
	}
	tracks, err := h.service.DeleteSubtitle(r.Context(), sessionFrom(r.Context()).User.ID, videoID, subtitleID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, tracks)
}
