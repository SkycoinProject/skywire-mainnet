package com.skywire.skycoin.vpn.controls;

import android.content.Context;
import android.content.res.TypedArray;
import android.util.AttributeSet;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.widget.FrameLayout;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ClickEvent;
import com.skywire.skycoin.vpn.helpers.BoxRowTypes;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;

public class BoxRowLayout extends FrameLayout implements ClickEvent {
    public BoxRowLayout(Context context) {
        super(context);
        Initialize(context, null);
    }
    public BoxRowLayout(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public BoxRowLayout(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private View baseBackground;
    private BoxRowBackground background;
    private BoxRowRipple ripple;
    private View separator;

    private ClickEvent clickListener;

    private boolean addExtraPaddingForTablets = false;
    private boolean ignoreMargins = false;
    private boolean ignoreClicks = false;
    private boolean hideSeparator = false;

    private int tabletExtraHorizontalPadding = 0;
    private float horizontalPadding;
    private float verticalPadding;

    private void Initialize (Context context, AttributeSet attrs) {
        baseBackground = new View(context);
        background = new BoxRowBackground(context);
        ripple = new BoxRowRipple(context);
        separator = new View(context);

        int type = 1;

        if (attrs != null) {
            TypedArray attributes = context.getTheme().obtainStyledAttributes(
                attrs,
                R.styleable.BoxRowLayout,
                0, 0
            );

            type = attributes.getInteger(R.styleable.BoxRowLayout_box_row_type, 1);

            addExtraPaddingForTablets = attributes.getBoolean(R.styleable.BoxRowLayout_add_extra_padding_for_tablets, false);
            ignoreMargins = attributes.getBoolean(R.styleable.BoxRowLayout_ignore_margins, false);
            ignoreClicks = attributes.getBoolean(R.styleable.BoxRowLayout_ignore_clicks, false);
            hideSeparator = attributes.getBoolean(R.styleable.BoxRowLayout_hide_separator, false);

            setUseBigFastClickPrevention(attributes.getBoolean(R.styleable.BoxRowLayout_use_big_fast_click_prevention, true));

            attributes.recycle();
        }

        horizontalPadding = TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_DIP,
            10,
            getResources().getDisplayMetrics()
        );
        if (!ignoreMargins) {
            horizontalPadding += getContext().getResources().getDimension(R.dimen.box_row_layout_horizontal_padding);
        }

        verticalPadding = 0;
        if (!ignoreMargins) {
            verticalPadding += getContext().getResources().getDimension(R.dimen.box_row_layout_vertical_padding);
        }

        if (addExtraPaddingForTablets) {
            tabletExtraHorizontalPadding = HelperFunctions.getTabletExtraHorizontalPadding(getContext());
        }

        separator.setBackgroundResource(R.color.box_separator);

        if (type == 0) {
            setType(BoxRowTypes.TOP);
        } else if (type == 1) {
            setType(BoxRowTypes.MIDDLE);
        } else if (type == 2) {
            setType(BoxRowTypes.BOTTOM);
        } else if (type == 3) {
            setType(BoxRowTypes.SINGLE);
        }

        this.setClipToPadding(false);

        this.addView(baseBackground);
        this.addView(background);
        if (!ignoreClicks) {
            ripple.setClickEventListener(this);
            this.addView(ripple);
        }
        this.addView(separator);

        setClickable(false);
    }

    public void setClickEventListener(ClickEvent listener) {
        clickListener = listener;
    }

    public void setUseBigFastClickPrevention(boolean useBigFastClickPrevention) {
        ripple.setUseBigFastClickPrevention(useBigFastClickPrevention);
    }

    public void setType(BoxRowTypes type) {
        float bottomPaddingExtra = 0;
        float topPaddingExtra = 0;

        if (type == BoxRowTypes.TOP) {
            baseBackground.setBackgroundResource(R.drawable.background_box1);

            topPaddingExtra = TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                10,
                getResources().getDisplayMetrics()
            );

            separator.setVisibility(View.VISIBLE);
        } else if (type == BoxRowTypes.MIDDLE) {
            baseBackground.setBackgroundResource(R.drawable.background_box2);
            separator.setVisibility(View.VISIBLE);
        } else if (type == BoxRowTypes.BOTTOM) {
            baseBackground.setBackgroundResource(R.drawable.background_box3);

            bottomPaddingExtra = TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                15,
                getResources().getDisplayMetrics()
            );

            separator.setVisibility(View.GONE);
        } else if (type == BoxRowTypes.SINGLE) {
            baseBackground.setBackgroundResource(R.drawable.background_box4);

            topPaddingExtra = TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                10,
                getResources().getDisplayMetrics()
            );
            bottomPaddingExtra = TypedValue.applyDimension(
                TypedValue.COMPLEX_UNIT_DIP,
                15,
                getResources().getDisplayMetrics()
            );

            separator.setVisibility(View.GONE);
        }

        if (hideSeparator) {
            separator.setVisibility(View.GONE);
        }

        int finalLeftPadding = (int)horizontalPadding;
        int finalTopPadding = (int)(verticalPadding + topPaddingExtra);
        int finalRightPadding = (int)horizontalPadding;
        int finalBottomPadding = (int)(verticalPadding + bottomPaddingExtra);

        this.setPadding(finalLeftPadding + tabletExtraHorizontalPadding, finalTopPadding, finalRightPadding + tabletExtraHorizontalPadding, finalBottomPadding);

        FrameLayout.LayoutParams backgroundLayoutParams = new FrameLayout.LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT);
        backgroundLayoutParams.leftMargin = -finalLeftPadding;
        backgroundLayoutParams.rightMargin = -finalRightPadding;
        if (finalTopPadding > 0) {
            backgroundLayoutParams.topMargin = -finalTopPadding;
        }
        if (finalBottomPadding > 0) {
            backgroundLayoutParams.bottomMargin = -finalBottomPadding;
        }

        baseBackground.setLayoutParams(backgroundLayoutParams);
        background.setLayoutParams(backgroundLayoutParams);
        background.setType(type);
        if (!ignoreClicks) {
            ripple.setLayoutParams(backgroundLayoutParams);
            ripple.setType(type);
        }

        float separatorHeight = getContext().getResources().getDimension(R.dimen.box_row_layout_separator_height);
        float separatorHorizontalMargin;
        if (ignoreMargins) {
            separatorHorizontalMargin = getContext().getResources().getDimension(R.dimen.box_row_layout_separator_combined_horizontal_margin);
        } else {
            separatorHorizontalMargin = getContext().getResources().getDimension(R.dimen.box_row_layout_separator_horizontal_margin);
        }

        FrameLayout.LayoutParams separatorLayoutParams = new FrameLayout.LayoutParams(LayoutParams.MATCH_PARENT, (int)Math.round(separatorHeight));
        separatorLayoutParams.gravity = Gravity.BOTTOM;
        separatorLayoutParams.bottomMargin = -finalBottomPadding;
        separatorLayoutParams.leftMargin = (int)separatorHorizontalMargin;
        separatorLayoutParams.rightMargin = (int)separatorHorizontalMargin;
        separator.setLayoutParams(separatorLayoutParams);
    }

    @Override
    public void onClick(View view) {
        if (clickListener != null) {
            clickListener.onClick(this);
        }
    }
}
