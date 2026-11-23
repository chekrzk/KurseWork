import { Link } from "react-router-dom";
import { Brand } from "./ui";
import { demoEnabled } from "./config";
export function Landing() {
  return (
    <main className="landing">
      <header>
        <Brand />
        <Link to="/login">Войти</Link>
      </header>
      <section>
        <h1>
          Короткие ссылки.
          <br />
          Измеримый результат.
        </h1>
        <p>
          Управляйте ссылками на своём домене, смотрите источники переходов и
          узнавайте, когда сайт перестал отвечать.
        </p>
        <div className="actions">
          <Link to="/register">Создать аккаунт ↗</Link>
          {demoEnabled && <Link to="/demo">Посмотреть демо</Link>}
        </div>
      </section>
      <div className="overview-detail">
        <article>
          <h2>Ссылки</h2>
          <p>Свой поддомен, UTM и лимит переходов.</p>
          <code>go.saturnita.ru/promo</code>
        </article>
        <article>
          <h2>Аналитика</h2>
          <p>Источники, устройства и ежедневная оценка числа посетителей.</p>
        </article>
        <article>
          <h2>Мониторинг</h2>
          <p>История HTTP-проверок, доступность и задержка p95.</p>
        </article>
      </div>
    </main>
  );
}
