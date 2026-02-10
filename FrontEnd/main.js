async function loadProfile() {
  const nameEl = document.getElementById("profileName");
  const emailEl = document.getElementById("profileEmail");
  const nameInput = document.getElementById("profileNameInput");
  const emailInput = document.getElementById("profileEmailInput");
  const roleEl = document.getElementById("profileRole");

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

    const deleteBtn = document.getElementById("deleteBtn");
    if (deleteBtn) deleteBtn.outerHTML = ``;

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
          method: "PUT",
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
          method: "PATCH",
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

    const me = await res.json();
    window.userID = me.id;
    document.body.style.display = "block";
  } catch (err) {
    console.error("Auth check failed", err);
    window.location.href = "/login";
  }
}

function setupDeleteAccount() {
  const deleteBtn = document.getElementById("deleteBtn");

  if (deleteBtn) {
    deleteBtn.addEventListener("click", async () => {
      if (!confirm("Are you sure you want to delete your account? This action cannot be undone.")) return;

      try {
        const res = await fetch("/api/delete-account", {
          method: "DELETE",
          credentials: "same-origin",
        });
        if (!res.ok) throw new Error("Failed to delete account");

        alert("Account deleted successfully");
        window.location.href = "/login";
      } catch (e) {
        console.error(e);
        alert("Failed to delete account");
      }
    });
  }
}

async function checkAdmin() {
    try {
        const res = await fetch("/api/me", { credentials: "same-origin" });
        if (!res.ok) throw new Error("Not authenticated");
        const user = await res.json();

        const roleEl = document.getElementById("profileRole");
        if (user.role === "admin" && roleEl) {
            roleEl.parentElement.style.display = "flex";
        }

        if (user.role === "admin") {
            document.querySelectorAll(".admin-nav").forEach(el => {
                el.style.display = "";
                el.style.color = "red";
            });
        }

    } catch (err) {
        console.error("Failed to fetch user info:", err);
    }
}

document.addEventListener("DOMContentLoaded", () => {
  loadProfile();
  setupProfileEdit();
  setupPhotoUpload();
  setupLogout();
  setupDeleteAccount();
  checkAdmin();
});
