function sendXHR(url, method, data, reload, failureMessage, successMessage, prompt) {
    let confirmed = true
    if (prompt != null) {
        confirmed = confirm(`Are you sure you want to (soft) delete '${url}'?`)
    }
    if (confirmed !== true) {
        return
    }

    let request = new XMLHttpRequest()
    request.open(method, url, false)

    request.addEventListener("loadend", function () {
        if (request.status !== 200) {
            alert(`${failureMessage} - status ${request.status} - ${request.response}`)
        } else {
            if (successMessage !== null) {
                alert(`${successMessage}`)
            }
            if (reload === true) {
                location.reload()
            }
        }
    })

    try {
        request.send(data)
    } catch (err) {
        alert(`${failureMessage} - exception '${err.message}'`)
    }
}

function controlAllCheckboxes(cb, className) {
    let checkboxes = document.getElementsByClassName(className)

    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].checked = cb.checked
    }
}

function batchDownloadFiles(checkboxClassName, attribute) {
    let checkboxes = document.getElementsByClassName(checkboxClassName)

    let url = "/submission-file-batch/"

    let checkedCounter = 0

    for (let i = 0; i < checkboxes.length; i++) {
        if (checkboxes[i].checked) {
            checkedCounter += 1
            url += checkboxes[i].dataset[attribute] + ","
        }
    }

    if (checkedCounter === 0) {
        alert("no submissions selected")
        return
    }

    url = url.slice(0, -1)
    window.location.href = url
}

function batchComment(checkboxClassName, attribute, action) {
    let checkboxes = document.getElementsByClassName(checkboxClassName);

    let url = "/submission-batch/"

    let checkedCounter = 0

    let magic = function (reload, successMessage) {
        url = url.slice(0, -1)

        let textArea = document.querySelector("#batch-comment-message")
        let ignoreDupesCheckbox = document.querySelector("#ignore-duplicate-actions")
        url += `/comment?action=${encodeURIComponent(action)}&message=${encodeURIComponent(textArea.value)}&ignore-duplicate-actions=${ignoreDupesCheckbox.checked}`

        sendXHR(url, "POST", null, reload, "Failed to post comment(s)", successMessage, null)
    }

    let u = new URL(window.location.href)

    // ugly black magic
    if (!u.href.endsWith("/submissions") && !u.href.endsWith("/my-submissions")) {
        url += checkboxes[0].dataset[attribute] + ","
        magic(true, null)
    } else {
        for (let i = 0; i < checkboxes.length; i++) {
            if (checkboxes[i].checked) {
                checkedCounter += 1
                url += checkboxes[i].dataset[attribute] + ","
            }
        }
        if (checkedCounter === 0) {
            alert("no submissions selected")
            return
        }
        magic(false, "Comments posted successfully")
    }
}

function changePage(number) {
    let url = new URL(window.location.href)

    let currentPage = url.searchParams.get("page")
    let newPage = 1 + number
    if (currentPage !== null) {
        let parsed = parseInt(currentPage, 10)
        if (!isNaN(parsed)) {
            newPage = parsed + number
        }
    }

    url.searchParams.set("page", newPage.toString())
    window.location.href = url
}

function uploadHandler(url, files, i) {
    if (i >= files.length) {
        document.querySelector("#upload-button").disabled = false
        return
    }
    let progressBarsContainer = document.querySelector("#progress-bars-container")

    let file = files[i]

    let formData = new FormData()
    formData.append("files", file)

    let progressBar = document.createElement("progress");
    progressBar.max = 100
    progressBar.value = 0
    let progressText = document.createElement("span");
    progressBarsContainer.appendChild(progressText)
    progressBarsContainer.appendChild(progressBar)

    let request = new XMLHttpRequest();
    request.open("POST", url)

    let t1 = 1
    let t2 = 2
    let p1 = 0
    let p2 = 0

    // upload progress event
    request.upload.addEventListener("progress", function (e) {
        t2 = performance.now()
        p2 = e.loaded
        let percent_complete = (e.loaded / e.total) * 100
        progressBar.value = percent_complete

        let uploadSpeed = ((((p2 - p1) / (t2 - t1)) * 1000) / 1000).toFixed(1)

        if (e.loaded === e.total) {
            progressText.innerHTML = `${file.name}<br>Processing and validating file, please wait...`
        } else {
            progressText.innerHTML = `${file.name}<br>Progress: ${percent_complete.toFixed(3)}% Upload speed: ${uploadSpeed}kB/s`
        }

        t1 = t2
        p1 = p2
    });

    let handleEnd = function (e) {
        if (request.status !== 200) {
            progressText.innerHTML = `${file.name}<br>something went wrong: status ${request.status} - ${request.response}`
            progressText.style.color = "red"
        } else {
            progressText.innerHTML = `${file.name}<br>Upload successful.`
        }
        uploadHandler(url, files, i + 1)
    }

    request.addEventListener("loadend", handleEnd);
    request.send(formData)
}

function bindFancierUpload(url) {
    let uploadButton = document.querySelector("#upload-button")
    let fileInput = document.querySelector("#file-input")

    uploadButton.addEventListener("click", function () {
        uploadButton.disabled = true

        if (fileInput.files.length === 0) {
            alert("Error : No file selected")
            uploadButton.disabled = false
            return
        }

        let files = fileInput.files
        for (let i = 0; i < files.length; i++) {
            if (!(files[i].name.endsWith(".7z") || files[i].name.endsWith(".zip"))) {
                alert('Error : Incorrect file type')
                uploadButton.disabled = false
                return
            }
        }

        uploadHandler(url, files, 0)
    });
}

function updateNotificationSettings() {
    let checkboxes = document.getElementsByClassName("notification-action")

    let url = "/api/notification-settings?"

    for (let i = 0; i < checkboxes.length; i++) {
        if (checkboxes[i].checked) {
            url += `notification-action=${encodeURIComponent(checkboxes[i].value)}` + "&"
        }
    }

    url = url.slice(0, -1)

    sendXHR(url, "PUT", null, true, "Failed to update notification settings", "Notification settings updated", null)
}

function updateSubscriptionSettings(sid, newValue) {
    sendXHR(`/submission/${sid}/subscription-settings?subscribe=${newValue}`, "PUT", null, true, "Failed to update subscription settings", null, null)
}

window.onload = function () {
    const images = document.getElementsByClassName('blur-img');
    for (let i = 0; i < images.length; i++) {
        images[i].addEventListener('click', toggleBlur);
    }


    function toggleBlur() {
        this.classList.toggle('blur-img');
    }
};

