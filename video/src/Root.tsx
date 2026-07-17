import React from 'react';
import { Composition } from 'remotion';
import { PlatformGoDemo } from './PlatformGoDemo';

export const RemotionRoot: React.FC = () => (
  <Composition
    id="PlatformGoDemo"
    component={PlatformGoDemo}
    durationInFrames={1620}
    fps={30}
    width={1920}
    height={1080}
    defaultProps={{}}
  />
);
