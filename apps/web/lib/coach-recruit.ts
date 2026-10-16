// コーチ募集の応募は、お問い合わせフォームに件名と本文の下書きを入れて受け付ける。
export const coachApplyTopic = "coach";
export const coachApplyPath = `/me/contact?topic=${coachApplyTopic}`;

export const coachApplyDraft = {
  category: "other",
  subject: "コーチに応募したい",
  body: [
    "応募する枠(コーチ／先輩):",
    "お名前(活動名):",
    "専門の種目:",
    "指導・競技の経験(自己ベストなど):",
    "YouTube や SNS の URL(あれば):",
    "ひとこと:",
  ].join("\n"),
} as const;
