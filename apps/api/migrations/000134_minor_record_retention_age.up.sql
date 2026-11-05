-- 未成年時の購入に紐づく記録（購入のたびの宣言、前の仕組みの保護者同意の証跡・購入の通知）は、
-- 退会しても本人が23歳になる月の終わりまで残す。未成年者取消しの取消権は成年になってから5年で消える（民法126条）ため、それに合わせる。
-- 退会でプロフィールが消えても数えられるよう、退会の時に生まれた年月を書き写す。
-- このマイグレーションより前に退会した分は生まれた年月がないので、従来どおり最後の取引から5年で消す。
ALTER TABLE minor_purchase_declarations ADD COLUMN birth_month date;
ALTER TABLE guardian_consents ADD COLUMN birth_month date;
