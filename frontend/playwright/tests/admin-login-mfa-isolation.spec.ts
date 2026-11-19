import { expect, test } from "@playwright/test";

test("senha e TOTP permanecem isolados durante a transição MFA", async ({ page }) => {
  let loginRequests = 0;
  const fakePassword = "FAKE-ONLY-password-value-123";
  const fakePasswords = [fakePassword, "FAKE-ONLY-second-attempt-456"];
  let fakePasswordInURL = false;
  page.on("request", request => {
    if (fakePasswords.some(value => new URL(request.url()).search.includes(value))) fakePasswordInURL = true;
  });
  await page.route("**/*", async route => {
    const { pathname } = new URL(route.request().url());
    if (route.request().method() === "POST" && pathname.endsWith("/session/mfa")) {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: { message: "Código inválido." } }),
      });
      return;
    }
    if (route.request().method() === "POST" && pathname.endsWith("/session")) {
      loginRequests += 1;
      if (loginRequests === 2) {
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({ error: { message: "Credenciais ou desafio inválidos." } }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            action: "mfa_setup_required",
            challenge_token: "frontend-test-challenge-only",
            setup_secret: "FAKE-ONLY-mfa-setup-secret",
            qr_code_data_url: "data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=",
          },
        }),
      });
      return;
    }
    await route.continue();
  });

  await page.goto("/admin/login", { waitUntil: "domcontentloaded" });
  const email = page.getByRole("textbox", { name: "E-mail" });
  const password = page.getByLabel("Senha");
  const continueButton = page.getByRole("button", { name: "Continuar" });
  await expect(continueButton).toBeEnabled();
  await expect(page.locator("form.admin-login-card")).toHaveAttribute("method", "post");
  await expect(email).toHaveValue("");
  await expect(password).toHaveAttribute("type", "password");
  await expect(password).toHaveAttribute("name", "password");
  await expect(password).toHaveAttribute("autocomplete", "current-password");
  await expect(password).toHaveValue("");
  await expect(page.getByLabel("Código do autenticador")).toHaveCount(0);

  await email.fill("mfa-isolation@example.invalid");
  await password.fill(fakePassword);
  expect(await page.locator("form").evaluate(form => (form as HTMLFormElement).checkValidity())).toBe(true);
  await continueButton.click();

  await expect.poll(() => loginRequests).toBe(1);
  const totp = page.getByLabel("Código do autenticador");
  await expect(page.getByRole("heading", { name: "Ativar MFA" })).toBeVisible();
  await expect(page.getByLabel("Senha")).toHaveCount(0);
  await expect(totp).toHaveAttribute("type", "text");
  await expect(totp).toHaveAttribute("name", "totp");
  await expect(totp).toHaveAttribute("inputmode", "numeric");
  await expect(totp).toHaveAttribute("autocomplete", "one-time-code");
  await expect(totp).toHaveAttribute("maxlength", "6");
  await expect(totp).toHaveValue("");
  expect(fakePasswordInURL).toBe(false);

  await totp.fill("123456");
  await expect(totp).toHaveValue("123456");
  await page.getByRole("button", { name: "Confirmar fator" }).click();
  await expect(page.locator("form.admin-login-card [role=alert]")).toHaveText("Código inválido.");
  await expect(totp).toHaveValue("");
  await expect(page.getByLabel("Senha")).toHaveCount(0);

  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(page.getByRole("button", { name: "Continuar" })).toBeEnabled();
  await expect(page.getByLabel("Senha")).toHaveValue("");
  await expect(page.getByLabel("Código do autenticador")).toHaveCount(0);

  await page.getByRole("textbox", { name: "E-mail" }).fill("mfa-isolation@example.invalid");
  const secondPassword = page.getByLabel("Senha");
  await secondPassword.fill(fakePasswords[1]);
  await page.getByRole("button", { name: "Continuar" }).click();
  await expect.poll(() => loginRequests).toBe(2);
  await expect(page.locator("form.admin-login-card [role=alert]")).toHaveText("Credenciais ou desafio inválidos.");
  await expect(secondPassword).toHaveValue("");
  await expect(page.getByLabel("Código do autenticador")).toHaveCount(0);

  await secondPassword.fill("FAKE-ONLY-third-attempt-789");
  fakePasswords.push("FAKE-ONLY-third-attempt-789");
  await page.getByRole("button", { name: "Continuar" }).click();
  await expect.poll(() => loginRequests).toBe(3);
  await expect(page.getByLabel("Código do autenticador")).toHaveValue("");
  expect(fakePasswordInURL).toBe(false);
});
