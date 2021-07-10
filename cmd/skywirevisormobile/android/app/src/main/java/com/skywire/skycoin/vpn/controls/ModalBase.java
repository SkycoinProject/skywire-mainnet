package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.TextView;

import com.skywire.skycoin.vpn.R;

public class ModalBase extends FrameLayout {
    public ModalBase(Context context) {
        super(context);
        Initialize(context, null);
    }
    public ModalBase(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public ModalBase(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private FrameLayout mainContainer;
    private TextView textTitle;
    private FrameLayout contentArea;

    private void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_modal_base, this, true);

        mainContainer = findViewById(R.id.mainContainer);
        textTitle = findViewById(R.id.textTitle);
        contentArea = findViewById(R.id.contentArea);

        mainContainer.setClipToOutline(true);

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.ModalBase,
                0, 0
            );

            String title = attributes.getString(R.styleable.ModalBase_title);
            if (title != null) {
                textTitle.setText(title);
            }

            boolean removeInternalPadding = attributes.getBoolean(R.styleable.ModalBase_remove_internal_padding, false);
            if (removeInternalPadding) {
                contentArea.setPadding(0, 0, 0, 0);
            }

            attributes.recycle();
        }
    }

    public void setTitle(int resourceId) {
        textTitle.setText(resourceId);
    }

    public void setTitleString(String title) {
        textTitle.setText(title);
    }

    @Override
    public void addView(View child) {
        if (contentArea != null) {
            contentArea.addView(child);
        } else {
            super.addView(child);
        }
    }

    @Override
    public void addView(View child, int index) {
        if (contentArea != null) {
            contentArea.addView(child, index);
        } else {
            super.addView(child, index);
        }
    }

    @Override
    public void addView(View child, ViewGroup.LayoutParams params) {
        if (contentArea != null) {
            contentArea.addView(child, params);
        } else {
            super.addView(child, params);
        }
    }

    @Override
    public void addView(View child, int width, int height) {
        if (contentArea != null) {
            contentArea.addView(child, width, height);
        } else {
            super.addView(child, width, height);
        }
    }

    @Override
    public void addView(View child, int index, ViewGroup.LayoutParams params) {
        if (contentArea != null) {
            contentArea.addView(child, index, params);
        } else {
            super.addView(child, index, params);
        }
    }
}
