using System;
using System.IO;
using UnityEngine;
using UnityEditor;

namespace Figma2UGUI
{
    [Serializable]
    public class UINode
    {
        public string id;
        public string name;
        public string type;
        public UINode[] children;
        public AbsoluteBoundingBox absoluteBoundingBox; // done
        public Fill[] fills;
        public FColor backgroundColor;
        public string characters;
        public Style style;
        public UIModify uiModify;
        public Grid[] layoutGrids;

        // public string blendMode;
        // public Constraints constraints; // done
        // public bool clipsContent;
        // public Fill[] background;
        // public Fill[] strokes;
        // public float strokeWeight;
        // public string strokeAlign;
        // public Effect[] effects;
        // public string transitionNodeID;
        // public float transitionDuration;
        // public string transitionEasing;
    }

    [Serializable]
    public class AbsoluteBoundingBox
    {
        public float x;
        public float y;
        public float width;
        public float height;

        public Vector2 GetPosition()
        {
            return new Vector2(x, y);
        }

        public Vector2 GetSize()
        {
            return new Vector2(width, height);
        }
    }

    [Serializable]
    public class Constraints
    {
        public string vertical;
        public string horizontal;
    }
    [Serializable]
    public class Fill
    {
        public string blendMode;
        public string visible;
        public string type;
        public FColor color;
        public string imageRef;
        public FVector[] gradientHandlePositions;
        public GradientStops[] gradientStops;
    }
    [Serializable]
    public class FColor
    {
        public float r;
        public float g;
        public float b;
        public float a;

        public UnityEngine.Color ToColor()
        {
            return new UnityEngine.Color(r, g, b, a);
        }
    }

    [Serializable]
    public class Grid
    {
        public string pattern;
        public float sectionSize;
        public bool visible;
        public FColor color;
        public string alignment;
        public int gutterSize;
        public float offset;
        public int count;
    }

    [Serializable]
    public class Effect
    {
        public string type;
        public bool visible;
        public FColor color;
        public string blendMode;
        public FVector offset;
        public float radius;
    }

    [Serializable]
    public class FVector
    {
        public float x;
        public float y;

        public Vector2 ToVector2()
        {
            return new Vector2(x, y);
        }
    }

    [Serializable]
    public class GradientStops
    {
        public FColor color;
        public float position;
    }

    [Serializable]
    public class Style
    {
        public string fontFamily;
        public string fontPostScriptName;
        public int fontWeight;
        public float fontSize;
        public string textAlignHorizontal;
        public string textAlignVertical;
        public float letterSpacing;
        public float lineHeightPx;
        public float lineHeightPercent;
        public string lineHeightUnit;
        public string textCase;
        public string textDecoration;
    }

    public enum FontWeight
    {
        Thin = 100,
        Light = 300,
        Regular = 400,
        Medium = 500,
        Bold = 700,
        Black = 900,
        ThinItalic = 100
    }
}
