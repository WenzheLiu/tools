const saveTextKey = "playground-save-text";
const saveUrlKey = "playground-save-url";

function submitTryIt() {

  let mask = document.getElementById("iframemask");
  mask.style.display = "block";

  var text = document.getElementById("textareaCode").value;
  var url = document.getElementById("resources-path").value;

  window.localStorage.setItem(saveTextKey, text)
  fetch("run?path=" + url, {
    method: "POST",
    body: text,

  }).then(res => {
    res.text().then(txt => {
      let ifr = document.createElement("iframe");
      ifr.setAttribute("frameborder", "0");
      ifr.setAttribute("id", "iframeResult");
      ifr.setAttribute("name", "iframeResult");

      let wrapper = document.getElementById("iframewrapper");
      wrapper.innerHTML = "";
      wrapper.appendChild(ifr);

      let ifrw = (ifr.contentWindow) ? ifr.contentWindow : (ifr.contentDocument.document) ? ifr.contentDocument.document : ifr.contentDocument;
      ifrw.document.open();
      ifrw.document.write(txt);
      ifrw.document.close();

      mask.style.display = "none";
    });
  })
}

document.addEventListener('DOMContentLoaded', function () {
  let savedURL = window.localStorage.getItem(saveUrlKey);
  if (savedURL == null || savedURL === "") {
    savedURL = window.location.href + "welcome";
  }
  document.getElementById("resources-path").value = savedURL;
  let savedText = window.localStorage.getItem(saveTextKey);
  if (savedText == null || savedText === "") {
    fetch("welcome/index.slides")
      .then(res => {
        res.text().then(body => {
          document.getElementById("textareaCode").value = body;
          submitTryIt();
        })
      })
  } else {
    document.getElementById("textareaCode").value = savedText;
    submitTryIt();
  }
}, false);