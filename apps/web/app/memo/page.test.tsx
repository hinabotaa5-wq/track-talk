/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import Page from "./page";
const fetch = vi.hoisted(() => vi.fn());
vi.mock("@/components/auth-provider", () => ({useAuth:()=>({session:{},token:"test",loading:false})}));
vi.mock("next/navigation", () => ({useRouter:()=>({push:vi.fn()})}));
vi.mock("@/lib/api", () => ({apiFetch:fetch,apiErrorText:()=>"失敗"}));
vi.mock("@/components/confirm-dialog", () => ({useConfirm:()=>vi.fn(),deleteConfirmOptions:vi.fn()}));
vi.mock("@/components/sense-note-jar", () => ({SenseNoteJar:()=>null}));
vi.mock("@/components/sense-notes", () => ({SenseNoteComposer:()=>null,SenseNoteStatsRow:()=>null,SenseNoteItem:({note}:{note:{body:string}})=><p>{note.body}</p>}));
vi.mock("@/components/view-switch", () => ({ViewSwitch:({views,onSelect}:{views:{id:string,label:string}[],onSelect:(id:string)=>void})=><nav>{views.map((v)=><button key={v.id} onClick={()=>onSelect(v.id)}>{v.label}</button>)}</nav>}));
vi.mock("next/link", () => ({default:({children,href}:{children:ReactNode,href:string})=><a href={href}>{children}</a>}));
const stats={total:1,today:1,days:1,streak:1};
const race = {id:"race",raced_on:"2026-10-24",event:"100m",event_label:"100m",mark:1100,wind:null,wind_aided:false,is_pb:true,memo:"大会の記録",meet_name:"秋季記録会",round:"",created_at:"2026-10-24T00:00:00Z"};
let root:Root, container:HTMLDivElement;
beforeEach(()=>{
 (globalThis as typeof globalThis & {IS_REACT_ACT_ENVIRONMENT:boolean}).IS_REACT_ACT_ENVIRONMENT=true;
 vi.useFakeTimers();vi.setSystemTime(new Date("2026-10-25T03:00:00Z"));
 fetch.mockReset();fetch.mockImplementation(async (path:string)=>path.includes("/race-records/pb-stats")?{events:[]}:path.includes("/me/race-records")?{records:[race],events:[{event:"100m",event_label:"100m",kind:"time",wind:true,count:1}]}:{notes:[{id:"note",day:"2026-10-24",body:"同じ日の練習の感覚",event:null,event_label:null}],stats,next_cursor:null});
 container=document.createElement("div");document.body.appendChild(container);root=createRoot(container);
});
afterEach(async()=>{await act(async()=>root.unmount());container.remove();vi.useRealTimers();});
it("感覚と記録のタブで同じ日・月とメモ・記録を共有し、選んだ日で記録を追加できる",async()=>{
 await act(async()=>root.render(<Page/>));
 await act(async()=>container.querySelector<HTMLButtonElement>('[aria-label="2026年10月24日、感覚メモあり、記録あり"]')!.click());
 expect(container.textContent).toContain("同じ日の練習の感覚");
 expect(container.textContent).toContain("大会の記録");
 await act(async()=>[...container.querySelectorAll("nav button")].find((b)=>b.textContent==="記録")!.dispatchEvent(new MouseEvent("click",{bubbles:true})));
 expect(container.querySelector('[aria-label="2026年10月24日、感覚メモあり、記録あり"]')!.getAttribute("aria-pressed")).toBe("true");
 expect(container.querySelector<HTMLInputElement>('input[type="date"]')!.value).toBe("2026-10-24");
 expect(container.textContent).toContain("同じ日の練習の感覚");
 expect(fetch.mock.calls.filter(([path])=>path==="/v1/me/race-records")).toHaveLength(1);
 await act(async()=>container.querySelector<HTMLButtonElement>('[aria-label="前の月"]')!.click());
 await act(async()=>[...container.querySelectorAll("nav button")].find((b)=>b.textContent==="感覚")!.dispatchEvent(new MouseEvent("click",{bubbles:true})));
 expect(container.textContent).toContain("2026年9月");
});
