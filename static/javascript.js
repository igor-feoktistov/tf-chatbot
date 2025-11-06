const EVENT_USER_PROMPT        = "01"
const EVENT_SYSTEM_PROMPT      = "02"
const EVENT_ASSISTANT_WAIT     = "03"
const EVENT_ASSISTANT_OUTPUT   = "04"
const EVENT_ASSISTANT_FINISH   = "05"
const EVENT_PING               = "06"
const EVENT_PONG               = "07"
const EVENT_DIAGNOSTIC         = "08"
const EVENT_CONFIRMED          = "09"
const EVENT_RESET_HISTORY      = "10"
const EVENT_ENABLE_HISTORY     = "11"
const EVENT_DISABLE_HISTORY    = "12"
const EVENT_CANCEL_USER_PROMPT = "14"
const EVENT_LOAD_SYSTEM_PROMPT = "15"

const robot = document.getElementById("robot");
const chatbox = document.getElementById('chatbox');
const systemPrompt = document.getElementById('systemPromptText');
const userPrompt = document.getElementById('userPromptText');
const connect = document.getElementById("connect");
const aboutWindow = document.getElementById("aboutWindow");

var ws = null;
var vs_protocol = "ws";

chatbox.innerHTML = `<h3>Assistant</h3></div>Hello! How can I help you today?`;
chatbox.scrollTop = chatbox.scrollHeight;

document.addEventListener("keydown", function(event) {
    if (event.key === "Enter" || event.key === "Escape") {
        if (aboutWindow.style.display === "flex") {
            aboutWindow.style.display = "none";
        }
    }
});

userPrompt.addEventListener('keydown', function(event) {
    if (event.key === "Enter") {
        event.preventDefault();
        sendUserPrompt();
    }
});

function startWebsocket() {
    if (window.location.protocol === "https:") {
        ws = new WebSocket("wss://" + window.location.host + "/ws");
    } else {
        ws = new WebSocket("ws://" + window.location.host + "/ws");
    }
    ws.addEventListener("open", (event) => {
        userPrompt.disabled = false;
        connect.style.visibility = "hidden";
        console.log('Websocket is connected!');
    });
    ws.addEventListener("message", (event) => {
        ws.onmessage = readMessage(event);
    });
    ws.addEventListener("close", (event) => {
        userPrompt.disabled = true;
        connect.style.visibility = "visible";
        checkWebsocket();
    });
}

function checkWebsocket(){
    if (!ws || ws.readyState == 3) startWebsocket();
}

function sendSystemPrompt() {
    const message = systemPrompt.value;
    if (message.trim().length > 0) {
        ws.send(EVENT_SYSTEM_PROMPT + ":" + message);
        document.getElementById('systemPromptDetails').open = false;
    }
}

function sendUserPrompt() {
    if (ws.readyState === WebSocket.CLOSED) {
        startWebsocket();
    }
    const message = userPrompt.value;
    if (message.trim().length > 0) {
        chatbox.innerHTML += `<div><h3>User</h3>${message}<h3>Assistant</h3></div>`;
        ws.send(EVENT_USER_PROMPT + ":" + message);
        userPrompt.value = "";
        chatbox.scrollTop = chatbox.scrollHeight;
        robot.src="/static/icons/robot_animated.gif";
        userPrompt.disabled = true;
    }
}

function cancelUserPrompt() {
    if (ws.readyState === WebSocket.CLOSED) {
        startWebsocket();
    }
    ws.send(EVENT_CANCEL_USER_PROMPT + ":");
}

function openAbout() {
    aboutWindow.style.display = "flex";
}

function closeAbout() {
    aboutWindow.style.display = "none";
}

function readMessage(event) {
    const eventType = event.data.substring(0, 2);
    switch (eventType) {
        case EVENT_ASSISTANT_WAIT:
            addSpinner();
            chatbox.scrollTop = chatbox.scrollHeight;
            break;
        case EVENT_ASSISTANT_OUTPUT:
        case EVENT_DIAGNOSTIC:
            deleteSpinner();
            chatbox.innerHTML += event.data.substring(3);
            chatbox.scrollTop = chatbox.scrollHeight;
            break;
        case EVENT_ASSISTANT_FINISH:
            deleteSpinner();
            robot.src="/static/icons/robot.png";
            userPrompt.disabled = false;
            break;
        case EVENT_LOAD_SYSTEM_PROMPT:
            systemPrompt.value = event.data.substring(3);
            break;
        case EVENT_CONFIRMED:
            const confirmedEventType = event.data.substring(3, 5);
            switch (confirmedEventType) {
                case EVENT_SYSTEM_PROMPT:
                    chatbox.innerHTML += `<p>Applied submitted by user system prompt.</p>`;
                    chatbox.scrollTop = chatbox.scrollHeight;
                    break;
                case EVENT_USER_PROMPT:
                    addSpinner();
                    chatbox.scrollTop = chatbox.scrollHeight;
                    break;
                case EVENT_RESET_HISTORY:
                    chatbox.innerHTML += `<p>Reset request history.</p>`;
                    chatbox.scrollTop = chatbox.scrollHeight;
                    break;
                case EVENT_ENABLE_HISTORY:
                    chatbox.innerHTML += `<p>Request history has been enabled.</p>`;
                    chatbox.scrollTop = chatbox.scrollHeight;
                    enableControlElement('disableHistory');
                    break;
                case EVENT_DISABLE_HISTORY:
                    chatbox.innerHTML += `<p>Request history has been disabled.</p>`;
                    chatbox.scrollTop = chatbox.scrollHeight;
                    enableControlElement('enableHistory');
                    break;
                case EVENT_CANCEL_USER_PROMPT:
                    chatbox.innerHTML += `<p>User request has been canceled.</p>`;
                    chatbox.scrollTop = chatbox.scrollHeight;
                    break;
                default:
                    console.log('Unexpected confirmed event type: ' + confirmedEventType);
                    console.log('event data: ' + event.data);
            }
            break;
        case EVENT_PING:
            ws.send(EVENT_PONG + ":" + 'pong');
            break;
        case EVENT_PONG:
            break;
        default:
            console.log("Unexpected event type: " + eventType);
    }
}

function addSpinner() {
    chatbox.innerHTML += `<div id="spinner"><img src="/static/icons/gear.gif">&nbsp;Awaiting response...</div>`;
}

function deleteSpinner() {
    const spinner = document.getElementById("spinner");
    if (spinner) {
        chatbox.removeChild(spinner);
    }
}

function enableControlElement(controlElement) {
    const element = document.getElementById(controlElement);
    if (element) {
        element.style.display = "flex";
    }
}

function disableControlElement(controlElement) {
    const element = document.getElementById(controlElement);
    if (element) {
        element.style.display = "none";
    }
}

function resetHistory() {
    if (ws.readyState === WebSocket.CLOSED) {
        startWebsocket();
    }
    ws.send(EVENT_RESET_HISTORY + ":");
}

function disableHistory() {
    disableControlElement("disableHistory");
    if (ws.readyState === WebSocket.CLOSED) {
        startWebsocket();
    }
    ws.send(EVENT_DISABLE_HISTORY + ":");
}

function enableHistory() {
    disableControlElement("enableHistory");
    if (ws.readyState === WebSocket.CLOSED) {
        startWebsocket();
    }
    ws.send(EVENT_ENABLE_HISTORY + ":");
}

function clearChat() {
    chatbox.innerHTML = `<h3>Assistant</h3></div>Hello! How can I help you today?`;
    chatbox.scrollTop = chatbox.scrollHeight;
}

function clearSystemPrompt() {
    systemPrompt.value = "";
}

function loadSystemPrompt() {
    if (ws.readyState === WebSocket.CLOSED) {
        startWebsocket();
    }
    ws.send(EVENT_LOAD_SYSTEM_PROMPT + ":");
}

function showTooltip(element, text, event) {
    const tooltip = document.createElement("div");
    tooltip.className = "dynamic-tooltip";
    tooltip.textContent = text;
    tooltip.id = "active-tooltip";
    document.body.appendChild(tooltip);
    const rect = element.getBoundingClientRect();
    let left = rect.left + rect.width / 2 - tooltip.offsetWidth / 2 + 15;
    let top = rect.top - tooltip.offsetHeight - 5;
    if (left < 10) {
        left = 10;
    }
    if ((left + tooltip.offsetWidth) > (window.innerWidth - 10)) {
        left = window.innerWidth - tooltip.offsetWidth - 10;
    }
    if (top < 10) {
        top = rect.bottom + 10;
    }
    tooltip.style.left = left + "px";
    tooltip.style.top = top + "px";
    setTimeout(() => {
        tooltip.classList.add("visible");
    }, 1);
}

function hideTooltip() {
    const tooltip = document.getElementById("active-tooltip");
    if (tooltip) {
        tooltip.classList.remove("visible");
        setTimeout(() => {
            tooltip.remove();
        }, 10);
    }
}

startWebsocket();
setInterval(checkWebsocket, 5000);

document.querySelectorAll("[data-tooltip]").forEach(element => {
    element.addEventListener("mouseenter", function(event) {
        const tooltipText = this.getAttribute("data-tooltip");
        showTooltip(this, tooltipText, event);
    });
    element.addEventListener("mouseleave", function() {
        hideTooltip();
    });
});

enableControlElement("disableHistory");
