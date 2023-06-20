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

async function sendXHR(url, method, data, reload, failureMessage, successMessage, promptMessage) {
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

    const res = await fetch(url, {
        method,
        body: data
    })
    if (res.ok) {
        if (successMessage) {
            return successMessage
        }
        if (reload) {
            location.reload()
        }
    } else {
        const responseText = await res.text();
        alert(`${failureMessage}\nRequest status: ${res.status} - ${res.statusText}\nRequest response: ${responseText}`);
    }
}

function controlAllCheckboxes(cb, className) {
    let checkboxes = document.getElementsByClassName(className)

    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].checked = cb.checked
    }

    updateBatchSize()
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

async function batchComment(checkboxClassName, attribute, action, body) {
    let checkboxes = document.getElementsByClassName(checkboxClassName);

    let url = "/api/submission-batch/"

    let checkedCounter = 0

    let magic = async function (reload, successMessage) {
        url = url.slice(0, -1)

        let textArea = document.querySelector("#batch-comment-message")
        let ignoreDupesCheckbox = document.querySelector("#ignore-duplicate-actions")
        let checked = false
        if (ignoreDupesCheckbox !== null) {
            checked = ignoreDupesCheckbox.checked
        }
        url += `/comment?action=${encodeURIComponent(action)}&message=${encodeURIComponent(textArea.value)}&ignore-duplicate-actions=${checked}`

        if (body) {
            await sendXHR(url, "POST", null, reload,
                `Failed to post comment(s) with action '${action}'.`, successMessage, null)
        } else {
            await sendXHR(url, "POST", body, reload,
                `Failed to post comment(s) with action '${action}'.`, successMessage, null)
        }

    }

    let u = new URL(window.location.href)

    // ugly black magic
    if (!u.pathname.endsWith("/submissions") && !u.pathname.endsWith("/my-submissions")) {
        url += checkboxes[0].dataset[attribute] + ","
        await magic(true, null)
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
        await magic(false, `Comments with action '${action}' posted successfully.`)
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

async function updateNotificationSettings() {
    let checkboxes = document.getElementsByClassName("notification-action")

    let url = "/api/notification-settings?"

    for (let i = 0; i < checkboxes.length; i++) {
        if (checkboxes[i].checked) {
            url += `notification-action=${encodeURIComponent(checkboxes[i].value)}` + "&"
        }
    }

    url = url.slice(0, -1)

    await sendXHR(url, "PUT", null, true,
        "Failed to update notification settings.",
        "Notification settings updated.", null)
}

async function updateSubscriptionSettings(sid, newValue) {
    await sendXHR(`/api/submission/${sid}/subscription-settings?subscribe=${newValue}`, "PUT", null, true,
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

    let checkboxes = document.getElementsByClassName("submission-checkbox")

    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].addEventListener('change', updateBatchSize)
    }

    updateBatchSize()
    wrapLongWordsInTable()
}

function updateBatchSize(event) {
    let sizeSpan = document.getElementById("submission-batch-size")
    if (sizeSpan !== null) {
        let checkboxes = document.getElementsByClassName("submission-checkbox")

        let totalSize = 0

        for (let i = 0; i < checkboxes.length; i++) {
            let cb = checkboxes[i]
            if (cb.checked) {
                let fid = cb.dataset.fid
                let tdSize = document.getElementById(`submission-file-size-${fid}`).dataset.size
                let parsed = parseInt(tdSize, 10)
                totalSize += parsed
            }
        }

        sizeSpan.innerText = `Total size of the selected batch: ${sizeToString(totalSize)}`
    }
}

function sizeToString(bytes, decimals = 1) {
    if (bytes === 0) return '0B';

    const k = 1000;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

    const i = Math.floor(Math.log(bytes) / Math.log(k));

    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + sizes[i];
}

async function deleteSubmissionFile(sid, sfid) {
    await sendXHR(`/api/submission/${sid}/file/${sfid}`, "DELETE", null, true,
        "Failed to delete submission file.",
        "Submission file deleted successfully.",
        "Please provide a reason to delete this submission file:")
}

async function deleteSubmission(sid) {
    await sendXHR(`/api/submission/${sid}`, "DELETE", null, true,
        "Failed to delete submission.",
        "Submission deleted successfully.",
        "Please provide a reason to delete this submission and all its related data:")
}

async function overrideBot(sid) {
    await sendXHR(`/api/submission/${sid}/override`, "POST", null, true,
        "Failed to override bot decision.",
        "Override successful.",
        null)
}

async function deleteComment(sid, cid) {
    await sendXHR(`/api/submission/${sid}/comment/${cid}`, "DELETE", null, true,
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

    if (formSimple !== null) {
        r(formSimple.getElementsByTagName("input"))
    }
    if (formAdvanced !== null) {
        r(formAdvanced.getElementsByTagName("input"))
    }
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
    document.getElementById("distinct-action-not-reject").checked = true
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
    document.getElementById("distinct-action-not-reject").checked = true
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
    document.getElementById("distinct-action-not-reject").checked = true
    document.getElementById("requested-changes-status-none").checked = true
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
        return
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
    let announcement = document.getElementById("system-announcement")
    if (announcement !== null) {
        announcement.style.maxWidth = maxWidth
    }
    document.getElementById("main").style.maxWidth = maxWidth
}

function wrapLongWordsInTable() {
    wrapLongWords(document.getElementsByClassName("submission-table-title"))
    wrapLongWords(document.getElementsByClassName("submission-table-original-filename"))
    wrapLongWords(document.getElementsByClassName("wrap-me"))
}

function wrapLongWords(list) {
    for (let i = 0; i < list.length; i++) {
        let longestWord = 0
        let title = list[i].innerHTML

        let currentWord = 0
        for (let j = 0; j < title.length; j++) {
            if (!/\s/.test(title[j])) {
                currentWord++
            } else {
                currentWord = 0
            }
            if (currentWord > longestWord) {
                longestWord = currentWord
            }
        }

        if (longestWord >= 32) {
            list[i].style.wordBreak = "break-all"
        }
    }
}

function linkIDsInComments() {
    const uuidRegex = /([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/g;
    let comments = document.getElementsByClassName("comment-body")

    for (let i = 0; i < comments.length; i++) {
        comments[i].innerHTML = comments[i].innerHTML.replaceAll(/(ID (\d+) )/g, '<a href="/web/submission/$2">$1</a>')
        comments[i].innerHTML = comments[i].innerHTML.replaceAll(uuidRegex, '<a href="/web/game/$1">$1</a>')
    }
}

function submitUUID(type) {
    let uuid = document.getElementById("uuid-input").value
    window.location.href = "/web/fixes/submit/" + uuid + "/" + type
}

function enableDarkMode() {
    document.getElementsByTagName("head")[0].insertAdjacentHTML(
        "beforeend",
        '<link id="dark-mode" rel="stylesheet" href="/static/dark.css"/>');
    document.getElementById("lights").innerText="Lights on"
    document.getElementById("lights").onclick=disableDarkMode
    localStorage.setItem("mode", "dark");
}

function disableDarkMode() {
    let e = document.getElementById("dark-mode")
    e.parentNode.removeChild(e)
    document.getElementById("lights").innerText="Lights off"
    document.getElementById("lights").onclick=enableDarkMode
    localStorage.setItem("mode", "light");
}

function setColors() {
    const mode = localStorage.getItem("mode");
    if (mode == null) {
        return
    }
    if (mode === "dark") {
        enableDarkMode()
    }
}

function populateUserStatisticsTable() {
    let request = new XMLHttpRequest()
    request.open("GET", "/api/users", true)

    request.addEventListener("loadend", function () {
        if (request.status !== 200) {
            return
        }
        
        let users = null
        try {
            users = JSON.parse(request.response)
        } catch (err) {
            console.error(err)
            alert("there was an error receiving the data, refresh the page to try again")
            return
        }

        processOneUserStatistics(users.users, 0)
    })


    try {
        request.send()
    } catch (err) {
        alert(`exception '${err.message}'`)
    }
}

function processOneUserStatistics(users, index) {
    if (index >= users.length) {
        return
    }

    let userID = users[index].id

    let request = new XMLHttpRequest()
    request.open("GET", `/api/user-statistics/${userID}`, true)

    request.addEventListener("loadend", function () {
        if (request.status === 200) {
            let stats = null
        try {
            stats = JSON.parse(request.response)
        } catch (err) {
            console.error(err)
            alert("there was an error receiving the data, refresh the page to try again")
            return
        }

        let table = document.getElementById("users-table")
        let row = table.insertRow(-1)

        // could this be done using some map fuckery? probably!
        let cell = row.insertCell(-1)
        cell.innerHTML = stats.UserID

        cell = row.insertCell(-1)
        cell.innerHTML = stats.Username

        cell = row.insertCell(-1)
        cell.innerHTML = stats.Role

        cell = row.insertCell(-1)
        cell.innerHTML = stats.LastUserActivity

        cell = row.insertCell(-1)
        cell.innerHTML = stats.UserCommentedCount

        cell = row.insertCell(-1)
        cell.innerHTML = stats.UserRequestedChangesCount
        cell.classList.add("bgr-request-changes")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.UserApprovedCount
        cell.classList.add("bgr-approve")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.UserVerifiedCount
        cell.classList.add("bgr-verify")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.UserAddedToFlashpointCount
        cell.classList.add("bgr-mark-added")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.UserRejectedCount
        cell.classList.add("bgr-reject")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsCount

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsBotHappyCount

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsBotUnhappyCount

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsRequestedChangesCount
        cell.classList.add("bgr-request-changes")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsApprovedCount
        cell.classList.add("bgr-approve")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsVerifiedCount
        cell.classList.add("bgr-verify")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsAddedToFlashpointCount
        cell.classList.add("bgr-mark-added")

        cell = row.insertCell(-1)
        cell.innerHTML = stats.SubmissionsRejectedCount
        cell.classList.add("bgr-reject")
        }

        processOneUserStatistics(users, index+1)
    })

    try {
        request.send()
    } catch (err) {
        alert(`exception '${err.message}'`)
    }
}

function doDeviceFlowAction(userCode, action) {
    fetch("/auth/device?code=" + userCode + "&action=" + action, { method: 'POST' }).then((res) => {
        location.reload()
    }).catch((err) => {
        alert(err)
    })
}

async function doWaitingSpinner(message, cb) {
    // Create a new element
    const overlay = document.createElement('div');
    overlay.classList.add('overlay');
    overlay.style.zIndex = "10000";

    // Append the overlay element to the body
    document.body.appendChild(overlay);

    // Create a message box element
    const messageBox = document.createElement('div');
    messageBox.classList.add('message-box');
    messageBox.innerHTML = `
      <div class="spinner"></div>
      <div class="message">${message}</div>`;

    // Append the message box element to the overlay
    overlay.appendChild(messageBox);

    // Close the overlay at the end of the function
    function closeOverlay() {
        // Remove the overlay element from the body
        document.body.removeChild(overlay);
    }
    await cb()
    .finally(() => {
        closeOverlay();
    });

}

async function confirmAction(message, cb) {
    const confirmation = confirm(message);

    if (confirmation) {
        return cb();
    }
}

async function selectReason(message, options, cb) {
    const createElem = (type, classNames, text) => {
        const elem = document.createElement(type);
        classNames.forEach((className) => elem.classList.add(className));
        if (text) elem.innerText = text;
        return elem;
    };

    const messageBox = createElem('div', ['message-box']);
    const messageBoxInner = createElem('div', [], message);
    messageBox.appendChild(messageBoxInner);

    const dropdown = createElem('select', []);
    options.forEach((option) => {
        const optionElem = createElem('option', [], option);
        optionElem.setAttribute('value', option);
        dropdown.appendChild(optionElem);
    });
    messageBox.appendChild(dropdown);

    const confirmButton = createElem('button', ['pure-button', 'pure-button-primary', 'button-approve'], 'Confirm');
    const cancelButton = createElem('button', ['pure-button', 'pure-button-primary', 'button-delete'], 'Cancel');
    const buttonBox = createElem('div', ['message-box-buttons']);
    buttonBox.append(confirmButton, cancelButton);
    messageBox.appendChild(buttonBox);

    const overlay = createElem('div', ['overlay']);
    overlay.style.zIndex = '10000';
    overlay.appendChild(messageBox);
    document.body.appendChild(overlay);

    await new Promise((resolve, reject) => {
        confirmButton.addEventListener('click', () => resolve(dropdown.value));
        cancelButton.addEventListener('click', reject);
    })
        .then((reason) => cb(reason))
        .catch(() => { /** ignore */ })
        .finally(() => {
            document.body.removeChild(overlay);
        });
}