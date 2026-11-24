import {expect,test} from "@playwright/test";
test("демонстрация Overview",async({page})=>{
 test.skip(process.env.TEST_DEMO!=="true","Требуется сборка с VITE_DEMO_MODE=true");
 await page.goto("/demo");
 await expect(page.getByText("Демонстрация",{exact:true})).toBeVisible();
 await expect(page.getByRole("cell",{name:"/launch"})).toBeVisible();
 await expect(page.getByText("525",{exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
});
