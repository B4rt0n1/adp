async function loadProfile() {
  const nameEl = document.getElementById("profileName");
  const emailEl = document.getElementById("profileEmail");
  const nameInput = document.getElementById("profileNameInput");
  const emailInput = document.getElementById("profileEmailInput");

  try {
    const res = await fetch("/api/me", { credentials: "same-origin" });
    if (!res.ok) throw new Error("Not authenticated");

    const user = await res.json();

    if (nameEl) nameEl.textContent = user.name;
    if (emailEl) emailEl.textContent = user.email;
    if (nameInput) nameInput.value = user.name;
    if (emailInput) emailInput.value = user.email;

    const photoSrc = user.photo ? "/Profile-Images/" + user.photo : "/Profile-Images/default.jpg";
    updateProfileImages(photoSrc);

  } catch (e) {
    if (nameEl) nameEl.textContent = "Guest";
    if (emailEl) emailEl.textContent = "-";

    const logoutBtn = document.getElementById("logoutBtn");
    if (logoutBtn) logoutBtn.outerHTML = `<a href="/login" class="primary-btn">Login</a>`;

    const changePhotoBtn = document.getElementById("changePhotoBtn");
    if (changePhotoBtn) changePhotoBtn.outerHTML = ``;

    const editBtn = document.getElementById("editBtn");
    if (editBtn) editBtn.outerHTML = ``;

    updateProfileImages("/Profile-Images/default.jpg");
  }
}

function updateProfileImages(photoUrl) {
  const profileImgs = document.querySelectorAll(".profile-img");
  profileImgs.forEach(img => {
    img.src = photoUrl + "?t=" + new Date().getTime();
  });

  const profileImg = document.getElementById("profile-image");
  if (profileImg) profileImg.src = photoUrl + "?t=" + new Date().getTime();
}

function setupProfileEdit() {
  const editBtn = document.getElementById("editBtn");
  const saveBtn = document.getElementById("saveBtn");
  const cancelBtn = document.getElementById("cancelBtn");
  const profileName = document.getElementById("profileName");
  const profileEmail = document.getElementById("profileEmail");
  const profileNameInput = document.getElementById("profileNameInput");
  const profileEmailInput = document.getElementById("profileEmailInput");

  if (editBtn && saveBtn && cancelBtn && profileName && profileEmail && profileNameInput && profileEmailInput) {

    editBtn.addEventListener("click", () => {
      profileName.style.display = "none";
      profileEmail.style.display = "none";
      profileNameInput.style.display = "inline-block";
      profileEmailInput.style.display = "inline-block";
      editBtn.style.display = "none";
      saveBtn.style.display = "inline-block";
      cancelBtn.style.display = "inline-block";
    });

    cancelBtn.addEventListener("click", () => {
      profileName.style.display = "inline";
      profileEmail.style.display = "inline";
      profileNameInput.style.display = "none";
      profileEmailInput.style.display = "none";
      editBtn.style.display = "inline-block";
      saveBtn.style.display = "none";
      cancelBtn.style.display = "none";
    });

    saveBtn.addEventListener("click", async () => {
      const updatedName = profileNameInput.value.trim();
      const updatedEmail = profileEmailInput.value.trim();

      if (!updatedName || !updatedEmail) {
        alert("Name and Email cannot be empty");
        return;
      }

      try {
        const res = await fetch("/api/update-profile", {
          method: "POST",
          credentials: "same-origin",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ name: updatedName, email: updatedEmail })
        });

        if (!res.ok) {
          const data = await res.json();
          throw new Error(data.error || "Update failed");
        }

        profileName.textContent = updatedName;
        profileEmail.textContent = updatedEmail;

        profileName.style.display = "inline";
        profileEmail.style.display = "inline";
        profileNameInput.style.display = "none";
        profileEmailInput.style.display = "none";
        editBtn.style.display = "inline-block";
        saveBtn.style.display = "none";
        cancelBtn.style.display = "none";

        alert("Profile updated successfully!");
      } catch (e) {
        alert(e.message);
        console.error(e);
      }
    });
  }
}

function setupPhotoUpload() {
  const changePhotoBtn = document.getElementById("changePhotoBtn");
  const profileFile = document.getElementById("profileFile");

  if (changePhotoBtn && profileFile) {
    changePhotoBtn.addEventListener("click", () => profileFile.click());

    profileFile.addEventListener("change", async () => {
      if (!profileFile.files || profileFile.files.length === 0) return;

      const formData = new FormData();
      formData.append("photo", profileFile.files[0]);

      try {
        const res = await fetch("/api/upload-photo", {
          method: "POST",
          credentials: "same-origin",
          body: formData
        });

        if (!res.ok) throw new Error("Upload failed");

        const data = await res.json();
        updateProfileImages(data.url);
        alert("Photo updated!");
      } catch (e) {
        console.error(e);
        alert("Failed to upload photo");
      }
    });
  }
}

function setupLogout() {
  const logoutBtn = document.getElementById("logoutBtn");
  if (logoutBtn) {
    logoutBtn.addEventListener("click", async () => {
      await fetch("/api/logout", { method: "POST", credentials: "same-origin" });
      window.location.href = "/login";
    });
  }
}

async function blockIfNotAuth() {
  try {
    const res = await fetch("/api/me", { credentials: "same-origin" });
    if (!res.ok) {
      window.location.href = "/login";
      return;
    }
    document.body.style.display = "block";
  } catch (err) {
    console.error("Auth check failed", err);
    window.location.href = "/login";
  }
}

loadProfile();
setupProfileEdit();
setupPhotoUpload();
setupLogout();
