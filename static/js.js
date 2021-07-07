let friendlyHttpStatus = {
    '200': 'OK',
    '201': 'Created',
    '202': 'Accepted',
    '203': 'Non-Authoritative Information',
    '204': 'No Content',
    '205': 'Reset Content',
    '206': 'Partial Content',
    '300': 'Multiple Choices',
    '301': 'Moved Permanently',
    '302': 'Found',
    '303': 'See Other',
    '304': 'Not Modified',
    '305': 'Use Proxy',
    '306': 'Unused',
    '307': 'Temporary Redirect',
    '400': 'Bad Request',
    '401': 'Unauthorized',
    '402': 'Payment Required',
    '403': 'Forbidden',
    '404': 'Not Found',
    '405': 'Method Not Allowed',
    '406': 'Not Acceptable',
    '407': 'Proxy Authentication Required',
    '408': 'Request Timeout',
    '409': 'Conflict',
    '410': 'Gone',
    '411': 'Length Required',
    '412': 'Precondition Required',
    '413': 'Request Entry Too Large',
    '414': 'Request-URI Too Long',
    '415': 'Unsupported Media Type',
    '416': 'Requested Range Not Satisfiable',
    '417': 'Expectation Failed',
    '418': 'I\'m a teapot',
    '429': 'Too Many Requests',
    '500': 'Internal Server Error',
    '501': 'Not Implemented',
    '502': 'Bad Gateway',
    '503': 'Service Unavailable',
    '504': 'Gateway Timeout',
    '505': 'HTTP Version Not Supported',
};

function sendXHR(url, method, data, reload, failureMessage, successMessage, promptMessage) {
    let reason = ""
    if (promptMessage != null) {
        reason = prompt(promptMessage)
        if (reason == null) {
            return
        }
        let urlObject = new URL(window.location.origin + url)
        urlObject.searchParams.set("reason", reason)
        url = urlObject.toString()
    }

    let request = new XMLHttpRequest()
    request.open(method, url, false)

    request.addEventListener("loadend", function () {
        if (request.status !== 200 && request.status !== 204) {
            alert(`${failureMessage}\nRequest status: ${request.status} - ${friendlyHttpStatus[request.status]}\nRequest response: ${request.response}`)
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

    let url = "/data/submission-file-batch/"

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

    let url = "/api/submission-batch/"

    let checkedCounter = 0

    let magic = function (reload, successMessage) {
        url = url.slice(0, -1)

        let textArea = document.querySelector("#batch-comment-message")
        let ignoreDupesCheckbox = document.querySelector("#ignore-duplicate-actions")
        let checked = false
        if (ignoreDupesCheckbox !== null) {
            checked = ignoreDupesCheckbox.checked
        }
        url += `/comment?action=${encodeURIComponent(action)}&message=${encodeURIComponent(textArea.value)}&ignore-duplicate-actions=${checked}`

        sendXHR(url, "POST", null, reload,
            `Failed to post comment(s) with action '${action}'.`, successMessage, null)
    }

    let u = new URL(window.location.href)

    // ugly black magic
    if (!u.pathname.endsWith("/submissions") && !u.pathname.endsWith("/my-submissions")) {
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
            alert("No submissions selected.")
            return
        }
        magic(false, `Comments with action '${action}' posted successfully.`)
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

function uploadHandler(url, files, i, step) {
    if (i >= files.length) {
        return
    }
    let progressBarsContainer = document.querySelector("#progress-bars-container")

    const file = files[i];

    let formData = new FormData()
    formData.append("files", file)

    let progressBar = document.createElement("progress");
    progressBar.max = 100
    progressBar.value = 0
    let progressText = document.createElement("span");
    progressText.style.fontWeight = "bold"
    progressText.style.fontSize = "90%"
    progressText.style.textShadow = "0 1px 1px rgba(0, 0, 0, 0.08)"
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
            progressText.innerHTML = `${file.name}<br>Upload failed!<br>Request status: ${request.status} - ${friendlyHttpStatus[request.status]}<br>Server response: ${request.response}`
            if (request.status === 409) {
                progressText.style.color = "orange"
            } else {
                progressText.style.color = "red"
            }
        } else {
            const obj = JSON.parse(request.response);

            if (obj["submission_ids"].length === 1) {
                progressText.innerHTML = `${file.name}<br>Upload successful. <a href="/web/submission/${obj["submission_ids"][0]}">View Submission</a>`
            } else {
                progressText.innerHTML = `${file.name}<br>Upload successful.`
            }
        }
        uploadHandler(url, files, i + step, step)
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

        let uploadQueues = document.querySelector("#upload-queues")
        for (let i = 0; i < parseInt(uploadQueues.value); i++) {
            uploadHandler(url, files, i, parseInt(uploadQueues.value))
        }
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

    sendXHR(url, "PUT", null, true,
        "Failed to update notification settings.",
        "Notification settings updated.", null)
}

function updateSubscriptionSettings(sid, newValue) {
    sendXHR(`/api/submission/${sid}/subscription-settings?subscribe=${newValue}`, "PUT", null, true,
        "Failed to update subscription settings.", null, null)
}

window.onload = function () {
    // blur pics
    const images = document.getElementsByClassName('blur-img');
    for (let i = 0; i < images.length; i++) {
        images[i].addEventListener('click', toggleBlur);
    }

    function toggleBlur() {
        this.classList.toggle('blur-img');
    }

    setSiteMaxWidth()
};

function deleteSubmissionFile(sid, sfid) {
    sendXHR(`/api/submission/${sid}/file/${sfid}`, "DELETE", null, true,
        "Failed to delete submission file.",
        "Submission file deleted successfully.",
        "Please provide a reason to delete this submission file:")
}

function deleteSubmission(sid) {
    sendXHR(`/api/submission/${sid}`, "DELETE", null, true,
        "Failed to delete submission.",
        "Submission deleted successfully.",
        "Please provide a reason to delete this submission and all its related data:")
}

function deleteComment(sid, cid) {
    sendXHR(`/api/submission/${sid}/comment/${cid}`, "DELETE", null, true,
        "Failed to delete comment.",
        null,
        "Please provide a reason to delete this comment:")
}

function resetFilterForm() {
    // default reset doesn't seem to work because i have divs inside the form
    let formSimple = document.getElementById("filter-form-simple")
    let formAdvanced = document.getElementById("filter-form-advanced")

    function r(inputs) {
        for (let i = 0; i < inputs.length; i++) {
            if (inputs[i].type === "checkbox" || inputs[i].type === "radio") {
                inputs[i].checked = false
            } else if (inputs[i].type === "text" || inputs[i].type === "number") {
                inputs[i].value = ""
            }
        }
    }

    r(formSimple.getElementsByTagName("input"))
    r(formAdvanced.getElementsByTagName("input"))
}

function submitAdvancedFilterForm() {
    document.getElementById("filter-form-advanced").submit()
}

function filterReadyForTesting() {
    resetFilterForm()
    let checkboxes = document.getElementsByClassName("bot-action-approve")
    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].checked = true
    }

    document.getElementById("approvals-status-none").checked = true
    document.getElementById("verification-status-none").checked = true
    document.getElementById("requested-changes-status-none").checked = true
    document.getElementById("assigned-status-testing-unassigned").checked = true
    document.getElementById("assigned-status-verification-unassigned").checked = true

    document.getElementById("last-uploader-not-me").checked = true
    document.getElementById("order-by-uploaded").checked = true
    document.getElementById("asc-desc-asc").checked = true

    document.getElementById("distinct-action-not-mark-added").checked = true
    submitAdvancedFilterForm()
}

function filterReadyForVerification() {
    resetFilterForm()
    let checkboxes = document.getElementsByClassName("bot-action-approve")
    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].checked = true
    }

    document.getElementById("approvals-status-approved").checked = true
    document.getElementById("requested-changes-status-none").checked = true
    document.getElementById("assigned-status-verification-unassigned").checked = true
    document.getElementById("verification-status-none").checked = true

    document.getElementById("approvals-status-me-no").checked = true
    document.getElementById("last-uploader-not-me").checked = true

    document.getElementById("order-by-uploaded").checked = true
    document.getElementById("asc-desc-asc").checked = true

    document.getElementById("distinct-action-not-mark-added").checked = true
    submitAdvancedFilterForm()
}


function filterReadyForFlashpoint() {
    resetFilterForm()
    let checkboxes = document.getElementsByClassName("bot-action-approve")
    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].checked = true
    }

    document.getElementById("verification-status-verified").checked = true

    document.getElementById("order-by-uploaded").checked = true
    document.getElementById("asc-desc-asc").checked = true

    document.getElementById("distinct-action-not-mark-added").checked = true
    submitAdvancedFilterForm()
}


function filterAssignedToMeForTesting() {
    resetFilterForm()

    document.getElementById("assigned-status-testing-me-assigned").checked = true

    submitAdvancedFilterForm()
}


function filterAssignedToMeForVerification() {
    resetFilterForm()

    document.getElementById("assigned-status-verification-me-assigned").checked = true

    submitAdvancedFilterForm()
}


function filterIHaveRequestedChangesAfterTesting() {
    resetFilterForm()

    document.getElementById("assigned-status-testing-me-assigned").checked = true
    document.getElementById("requested-changes-status-me-ongoing").checked = true

    submitAdvancedFilterForm()
}

function filterIHaveRequestedChangesVerification() {
    resetFilterForm()

    document.getElementById("assigned-status-verification-me-assigned").checked = true
    document.getElementById("requested-changes-status-me-ongoing").checked = true

    submitAdvancedFilterForm()
}

function switchFilterLayout(newLayout) {
    let url = new URL(window.location.href)

    keys = []
    for (let pair of url.searchParams.entries()) {
        keys.push(pair[0])
    }

    for (let k of keys) {
        url.searchParams.delete(k)
    }

    url.searchParams.set("filter-layout", newLayout)
    window.location.href = url
}

function updateLocalSettings() {
    const maxWidthInput = document.getElementById("site-max-width")
    let parsed = parseInt(maxWidthInput.value, 10)
    if (isNaN(parsed)) {
        parsed = 1300
    }
    localStorage.setItem("site-max-width", parsed.toString())
    setSiteMaxWidth()
    maxWidthInput.value = ""
    maxWidthInput.placeholder = parsed.toString()
}

function setSiteMaxWidth() {
    let maxWidth = localStorage.getItem("site-max-width")
    if (maxWidth === null) {
        maxWidth = "1300"
    }
    maxWidth += "px"
    document.getElementById("navbar").style.maxWidth = maxWidth
    document.getElementById("system-announcement").style.maxWidth = maxWidth
    document.getElementById("main").style.maxWidth = maxWidth
}