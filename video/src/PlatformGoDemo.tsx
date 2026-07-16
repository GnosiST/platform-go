import React from 'react';
import { AbsoluteFill, Easing, interpolate, Sequence, useCurrentFrame, useVideoConfig } from 'remotion';

const navy = '#0d1728';
const blue = '#4f8ff7';
const teal = '#5ee0c3';
const paper = '#f7f9fc';

const Fade: React.FC<{ children: React.ReactNode; from?: number }> = ({ children, from = 0 }) => {
  const frame = useCurrentFrame();
  const opacity = interpolate(frame, [from, from + 18], [0, 1], { extrapolateRight: 'clamp', extrapolateLeft: 'clamp', easing: Easing.out(Easing.cubic) });
  const translate = interpolate(frame, [from, from + 24], [28, 0], { extrapolateRight: 'clamp', extrapolateLeft: 'clamp', easing: Easing.out(Easing.cubic) });
  return <div style={{ opacity, translate: `0px ${translate}px` }}>{children}</div>;
};

const Label: React.FC<{ children: React.ReactNode }> = ({ children }) => <div style={{ color: teal, fontFamily: 'monospace', fontSize: 24, letterSpacing: 4, textTransform: 'uppercase' }}>{children}</div>;

const FlowScene: React.FC = () => {
  const frame = useCurrentFrame();
  const progress = interpolate(frame, [0, 120], [0, 1], { extrapolateRight: 'clamp', extrapolateLeft: 'clamp' });
  const nodes = [['01', 'Identity', '可信身份'], ['02', 'Capability', '服务合同'], ['03', 'Policy', '租户与权限'], ['04', 'Runtime', 'GORM 数据层']];
  return <AbsoluteFill style={{ backgroundColor: paper, padding: '110px 150px', color: navy }}><Fade><Label>platform-go / runtime path</Label><h2 style={{ fontSize: 74, margin: '28px 0 70px', letterSpacing: -3 }}>请求先过边界，数据才进入运行时。</h2></Fade><div style={{ display: 'flex', flexDirection: 'column', gap: 18, width: 1120 }}>{nodes.map(([num, title, caption], index) => <React.Fragment key={num}><div style={{ display: 'grid', gridTemplateColumns: '110px 1fr 1fr', alignItems: 'center', padding: '26px 34px', border: `2px solid ${index === 2 ? blue : '#d8e1eb'}`, backgroundColor: index === 2 ? '#edf4ff' : '#fff', borderRadius: 12, opacity: interpolate(progress, [index * .16, index * .16 + .2], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }) }}><b style={{ color: blue, fontFamily: 'monospace', fontSize: 28 }}>{num}</b><strong style={{ fontSize: 38 }}>{title}</strong><span style={{ fontSize: 27, color: '#66758b' }}>{caption}</span></div>{index < 3 && <div style={{ width: 3, height: 24, backgroundColor: '#b5c7dc', marginLeft: 52 }} />}</React.Fragment>)}</div><div style={{ position: 'absolute', right: 150, bottom: 110, color: blue, fontFamily: 'monospace', fontSize: 26 }}>ctx → contract → scope → store</div></AbsoluteFill>;
};

const ContractScene: React.FC = () => <AbsoluteFill style={{ backgroundColor: navy, color: '#f3f7fb', padding: '120px 150px' }}><Fade><Label>capability manifest</Label><h2 style={{ fontSize: 78, maxWidth: 1200, margin: '32px 0 64px', letterSpacing: -3 }}>把资源、权限和生命周期写进一份可验证的合同。</h2></Fade><Fade from={18}><div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24, maxWidth: 1400 }}><pre style={{ margin: 0, padding: 36, border: '1px solid #2d415e', borderRadius: 12, backgroundColor: '#111f34', color: '#b9d8ff', fontSize: 27, lineHeight: 1.6 }}>{`{\n  "id": "policy-review",\n  "stability": "stable",\n  "permissions": ["review:approve"],\n  "lifecycle": "soft-delete"\n}`}</pre><div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 24, fontSize: 34 }}><div><span style={{ color: teal }}>✓</span> OpenAPI 生成</div><div><span style={{ color: teal }}>✓</span> 前后端资源一致</div><div><span style={{ color: teal }}>✓</span> 发布证据可追溯</div></div></div></Fade></AbsoluteFill>;

const CloseScene: React.FC = () => { const { fps } = useVideoConfig(); return <AbsoluteFill style={{ backgroundColor: blue, color: '#fff', alignItems: 'center', justifyContent: 'center', textAlign: 'center' }}><Sequence from={0} durationInFrames={fps * 4}><Fade><div style={{ fontSize: 34, fontFamily: 'monospace', letterSpacing: 4 }}>BUSINESS-NEUTRAL / OPEN BY DESIGN</div><h2 style={{ fontSize: 100, margin: '28px 0 18px', letterSpacing: -4 }}>先建底座，再接业务。</h2><p style={{ fontSize: 34, margin: 0 }}>platform-go · Gin · GORM · Casbin · React</p></Fade></Sequence></AbsoluteFill>; };

export const PlatformGoDemo: React.FC = () => <AbsoluteFill style={{ fontFamily: 'Arial, sans-serif' }}><Sequence from={0} durationInFrames={150}><FlowScene /></Sequence><Sequence from={150} durationInFrames={120}><ContractScene /></Sequence><Sequence from={270} durationInFrames={90}><CloseScene /></Sequence></AbsoluteFill>;
