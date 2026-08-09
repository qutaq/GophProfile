(() => {
  const form = document.getElementById("upload-form");
  if (!form) return;

  const zone = document.getElementById("dropzone");
  const input = document.getElementById("file-input");
  const preview = document.getElementById("preview");
  const previewImg = document.getElementById("preview-img");
  const previewName = document.getElementById("preview-name");
  const progress = document.getElementById("progress");
  const bar = document.getElementById("progress-bar");
  const submit = document.getElementById("submit-btn");
  const flash = document.getElementById("flash");

  const showFlash = (type, text) => {
    if (!flash) return;
    flash.className = `flash ${type}`;
    flash.textContent = text;
    flash.hidden = false;
  };

  const setFile = (file) => {
    if (!file) return;
    const dt = new DataTransfer();
    dt.items.add(file);
    input.files = dt.files;
    previewImg.src = URL.createObjectURL(file);
    previewName.textContent = `${file.name} · ${(file.size / 1024).toFixed(1)} KB`;
    preview.classList.add("is-visible");
  };

  zone?.addEventListener("click", () => input.click());
  input?.addEventListener("change", () => setFile(input.files?.[0]));

  ["dragenter", "dragover"].forEach((evt) => {
    zone?.addEventListener(evt, (e) => {
      e.preventDefault();
      zone.classList.add("is-drag");
    });
  });
  ["dragleave", "drop"].forEach((evt) => {
    zone?.addEventListener(evt, (e) => {
      e.preventDefault();
      zone.classList.remove("is-drag");
    });
  });
  zone?.addEventListener("drop", (e) => {
    const file = e.dataTransfer?.files?.[0];
    setFile(file);
  });

  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const userID = form.user_id.value.trim();
    const file = input.files?.[0];
    if (!userID) {
      showFlash("err", "Укажите User ID");
      return;
    }
    if (!file) {
      showFlash("err", "Выберите изображение");
      return;
    }

    const data = new FormData();
    data.append("file", file);

    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/web/upload");
    xhr.setRequestHeader("X-User-ID", userID);

    progress.classList.add("is-visible");
    bar.style.width = "0%";
    submit.disabled = true;

    xhr.upload.onprogress = (ev) => {
      if (!ev.lengthComputable) return;
      bar.style.width = `${Math.round((ev.loaded / ev.total) * 100)}%`;
    };

    xhr.onload = () => {
      submit.disabled = false;
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const res = JSON.parse(xhr.responseText);
          window.location.href = `/web/gallery/${encodeURIComponent(userID)}?uploaded=${encodeURIComponent(res.id || "")}`;
          return;
        } catch (_) {
          window.location.href = `/web/gallery/${encodeURIComponent(userID)}`;
          return;
        }
      }
      let msg = "Ошибка загрузки";
      try {
        const res = JSON.parse(xhr.responseText);
        msg = res.error || msg;
        if (res.details) msg += `: ${res.details}`;
      } catch (_) {}
      showFlash("err", msg);
      progress.classList.remove("is-visible");
    };

    xhr.onerror = () => {
      submit.disabled = false;
      progress.classList.remove("is-visible");
      showFlash("err", "Сеть недоступна");
    };

    xhr.send(data);
  });
})();
