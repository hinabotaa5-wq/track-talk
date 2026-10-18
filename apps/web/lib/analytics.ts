// Umami に送る前に、ページの URL から秘密を外す。
// 保護者のページ(/guardian/{token})のトークンは、知っていれば保護者として同意やメッセージができてしまう。
// Umami の data-before-send は関数の名前で受け取るので、window に置く。インラインの script で読み込ませる。
export const umamiBeforeSendName = "umamiBeforeSend";

export const umamiBeforeSendScript = `window.${umamiBeforeSendName}=function(type,payload){
  var hide=function(v){return typeof v==="string"?v.replace(/\\/guardian\\/(?!paid(?:[/?#]|$))[^/?#]+/g,"/guardian/:token"):v;};
  if(payload){payload.url=hide(payload.url);payload.referrer=hide(payload.referrer);}
  return payload;
};`;
